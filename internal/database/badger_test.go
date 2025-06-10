/*
Copyright 2020 The Flux authors

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
package database

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/dgraph-io/badger/v3"
)

const testRepo = "testing/testing"

func TestGetWithUnknownRepo(t *testing.T) {
	db := createBadgerDatabase(t)

	tags, err := db.Tags(testRepo)
	fatalIfError(t, err)

	if !reflect.DeepEqual([]string{}, tags) {
		t.Fatalf("Tags() for unknown repo got %#v, want %#v", tags, []string{})
	}
}

func TestSetTags(t *testing.T) {
	db := createBadgerDatabase(t)
	tags := []string{"latest", "v0.0.1", "v0.0.2"}

	fatalIfError(t, setTags(t, db, testRepo, tags, "1943865137"))

	loaded, err := db.Tags(testRepo)
	fatalIfError(t, err)
	if !reflect.DeepEqual(tags, loaded) {
		t.Fatalf("SetTags failed, got %#v want %#v", loaded, tags)
	}
}

func TestSetTagsOverwrites(t *testing.T) {
	db := createBadgerDatabase(t)
	tags1 := []string{"latest", "v0.0.1", "v0.0.2"}
	tags2 := []string{"latest", "v0.0.1", "v0.0.2", "v0.0.3"}
	fatalIfError(t, setTags(t, db, testRepo, tags1, "1943865137"))

	fatalIfError(t, setTags(t, db, testRepo, tags2, "3168012550"))

	loaded, err := db.Tags(testRepo)
	fatalIfError(t, err)
	if !reflect.DeepEqual(tags2, loaded) {
		t.Fatalf("failed to overwrite with SetTags: got %#v, want %#v", loaded, tags2)
	}
}

func TestGetOnlyFetchesForRepo(t *testing.T) {
	db := createBadgerDatabase(t)
	tags1 := []string{"latest", "v0.0.1", "v0.0.2"}
	fatalIfError(t, setTags(t, db, testRepo, tags1, "1943865137"))
	testRepo2 := "another/repo"
	tags2 := []string{"v0.0.3", "v0.0.4"}
	fatalIfError(t, setTags(t, db, testRepo2, tags2, "728958008"))

	loaded, err := db.Tags(testRepo)
	fatalIfError(t, err)
	if !reflect.DeepEqual(tags1, loaded) {
		t.Fatalf("Tags() failed got %#v, want %#v", loaded, tags2)
	}
}

func createBadgerDatabase(t *testing.T) *BadgerDatabase {
	t.Helper()
	dir, err := os.MkdirTemp(os.TempDir(), "badger")
	if err != nil {
		t.Fatal(err)
	}
	db, err := badger.Open(badger.DefaultOptions(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return NewBadgerDatabase(db)
}

func setTags(t *testing.T, db *BadgerDatabase, repo string, tags []string, expectedChecksum string) error {
	t.Helper()
	checksum, err := db.SetTags(repo, tags)
	if err != nil {
		return err
	}
	if checksum != expectedChecksum {
		return fmt.Errorf("SetTags returned unexpected checksum: got %s, want %s", checksum, expectedChecksum)
	}
	return nil
}

func fatalIfError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
