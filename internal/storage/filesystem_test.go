/*
Copyright 2026 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	artifactstorage "github.com/fluxcd/pkg/artifact/storage"
)

func TestFilesystemDatabaseSetTagsPlain(t *testing.T) {
	db, st := newFilesystemDatabase(t, 1024)
	repo := testRepoIdentity("default", "podinfo")
	tags := []string{"latest", "v1.0.0"}

	revision, err := db.SetTags(context.Background(), repo, tags)
	if err != nil {
		t.Fatal(err)
	}
	if revision == "" {
		t.Fatal("SetTags returned empty revision")
	}
	if !pathExistsForTest(t, st.LocalPath(artifactForRepo(repo, tagsFilePlain))) {
		t.Fatal("plain tags file was not written")
	}
	if pathExistsForTest(t, st.LocalPath(artifactForRepo(repo, tagsFileGzip))) {
		t.Fatal("gzip tags file was written below threshold")
	}

	loaded, err := db.Tags(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tags, loaded) {
		t.Fatalf("Tags() got %#v, want %#v", loaded, tags)
	}
}

func TestFilesystemDatabaseSetTagsCompressed(t *testing.T) {
	db, st := newFilesystemDatabase(t, 1)
	repo := testRepoIdentity("default", "podinfo")
	tags := []string{"latest", "v1.0.0"}

	if _, err := db.SetTags(context.Background(), repo, tags); err != nil {
		t.Fatal(err)
	}
	if pathExistsForTest(t, st.LocalPath(artifactForRepo(repo, tagsFilePlain))) {
		t.Fatal("plain tags file was written above threshold")
	}
	if !pathExistsForTest(t, st.LocalPath(artifactForRepo(repo, tagsFileGzip))) {
		t.Fatal("gzip tags file was not written")
	}

	loaded, err := db.Tags(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tags, loaded) {
		t.Fatalf("Tags() got %#v, want %#v", loaded, tags)
	}
}

func TestFilesystemDatabaseEmptyAndMissing(t *testing.T) {
	db, _ := newFilesystemDatabase(t, 1)
	repo := testRepoIdentity("default", "podinfo")

	missing, err := db.Tags(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("Tags() for missing repo got %#v, want empty", missing)
	}

	if _, err := db.SetTags(context.Background(), repo, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.Tags(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Fatalf("Tags() for empty repo got %#v, want empty", loaded)
	}
}

func TestFilesystemDatabaseStaleVariantCleanup(t *testing.T) {
	db, st := newFilesystemDatabase(t, 1)
	repo := testRepoIdentity("default", "podinfo")
	if _, err := db.SetTags(context.Background(), repo, []string{"latest", "v1.0.0"}); err != nil {
		t.Fatal(err)
	}

	db.compressionThreshold = 1024
	if _, err := db.SetTags(context.Background(), repo, []string{"latest"}); err != nil {
		t.Fatal(err)
	}
	if !pathExistsForTest(t, st.LocalPath(artifactForRepo(repo, tagsFilePlain))) {
		t.Fatal("plain tags file was not written")
	}
	if pathExistsForTest(t, st.LocalPath(artifactForRepo(repo, tagsFileGzip))) {
		t.Fatal("stale gzip tags file was not removed")
	}
}

func TestFilesystemDatabaseBothVariantsPresentUsesNewest(t *testing.T) {
	db, st := newFilesystemDatabase(t, 1024)
	repo := testRepoIdentity("default", "podinfo")
	plainTags := []string{"old"}
	gzipTagsSet := []string{"new"}

	plainArtifact := artifactForRepo(repo, tagsFilePlain)
	gzipArtifact := artifactForRepo(repo, tagsFileGzip)
	if err := st.MkdirAll(plainArtifact); err != nil {
		t.Fatal(err)
	}
	plainPath := st.LocalPath(plainArtifact)
	gzipPath := st.LocalPath(gzipArtifact)
	if err := os.WriteFile(plainPath, marshalTagsLines(plainTags), 0o600); err != nil {
		t.Fatal(err)
	}
	compressed, err := gzipTags(marshalTagsLines(gzipTagsSet))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gzipPath, compressed, 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(plainPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(gzipPath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	loaded, err := db.Tags(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gzipTagsSet, loaded) {
		t.Fatalf("Tags() got %#v, want %#v", loaded, gzipTagsSet)
	}
	if pathExistsForTest(t, plainPath) {
		t.Fatal("older stale variant was not removed")
	}
}

func TestFilesystemDatabaseRevisionStableAcrossCompression(t *testing.T) {
	_, st := newFilesystemDatabase(t, 1)
	repo := testRepoIdentity("default", "podinfo")
	tags := []string{"latest", "v1.0.0"}

	compressedDB := NewFilesystemDatabase(st, 1)
	compressedRevision, err := compressedDB.SetTags(context.Background(), repo, tags)
	if err != nil {
		t.Fatal(err)
	}
	plainDB := NewFilesystemDatabase(st, 1024)
	plainRevision, err := plainDB.SetTags(context.Background(), repo, tags)
	if err != nil {
		t.Fatal(err)
	}
	if compressedRevision != plainRevision {
		t.Fatalf("revision changed across compression threshold: %s != %s", compressedRevision, plainRevision)
	}
}

func TestFilesystemDatabaseNamespaceNameIsolation(t *testing.T) {
	db, _ := newFilesystemDatabase(t, 1024)
	repoA := RepoIdentity{Namespace: "team-a", Name: "app", CanonicalName: "example.com/app"}
	repoB := RepoIdentity{Namespace: "team-b", Name: "app", CanonicalName: "example.com/app"}

	if _, err := db.SetTags(context.Background(), repoA, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetTags(context.Background(), repoB, []string{"b"}); err != nil {
		t.Fatal(err)
	}

	loadedA, err := db.Tags(context.Background(), repoA)
	if err != nil {
		t.Fatal(err)
	}
	loadedB, err := db.Tags(context.Background(), repoB)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]string{"a"}, loadedA) {
		t.Fatalf("repo A tags got %#v", loadedA)
	}
	if !reflect.DeepEqual([]string{"b"}, loadedB) {
		t.Fatalf("repo B tags got %#v", loadedB)
	}
}

func TestFilesystemDatabaseDelete(t *testing.T) {
	db, st := newFilesystemDatabase(t, 1024)
	repo := testRepoIdentity("default", "podinfo")
	if _, err := db.SetTags(context.Background(), repo, []string{"latest"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if pathExistsForTest(t, filepath.Dir(st.LocalPath(artifactForRepo(repo, tagsFilePlain)))) {
		t.Fatal("repo directory still exists after delete")
	}
}

func newFilesystemDatabase(t *testing.T, threshold int) (*FilesystemDatabase, *artifactstorage.Storage) {
	t.Helper()
	st := &artifactstorage.Storage{BasePath: t.TempDir()}
	return NewFilesystemDatabase(st, threshold), st
}

func testRepoIdentity(namespace, name string) RepoIdentity {
	return RepoIdentity{Namespace: namespace, Name: name, CanonicalName: "example.com/" + name}
}

func pathExistsForTest(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatal(err)
	return false
}

func TestGzipTagsDeterministic(t *testing.T) {
	data := marshalTagsLines([]string{"latest", "v1.0.0"})
	first, err := gzipTags(data)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gzipTags(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("gzip output is not deterministic")
	}

	reader, err := gzip.NewReader(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	if reader.Name != "" || !reader.ModTime.IsZero() {
		t.Fatalf("gzip header contains name or modtime: name=%q modtime=%s", reader.Name, reader.ModTime)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}
