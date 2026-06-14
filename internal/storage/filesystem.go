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
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/fluxcd/pkg/apis/meta"
	artifactstorage "github.com/fluxcd/pkg/artifact/storage"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
)

const (
	tagsFilePlain = "tags.txt"
	tagsFileGzip  = "tags.txt.gz"
)

// FilesystemDatabase stores image tags per ImageRepository on the local filesystem.
type FilesystemDatabase struct {
	storage              *artifactstorage.Storage
	compressionThreshold int
}

// NewFilesystemDatabase creates a filesystem-backed tag database.
func NewFilesystemDatabase(storage *artifactstorage.Storage, compressionThreshold int) *FilesystemDatabase {
	return &FilesystemDatabase{
		storage:              storage,
		compressionThreshold: compressionThreshold,
	}
}

// Tags implements the DatabaseReader interface, fetching tags for the repo.
func (d *FilesystemDatabase) Tags(ctx context.Context, repo RepoIdentity) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateRepoIdentity(repo); err != nil {
		return nil, err
	}

	plain, err := d.statVariant(repo, tagsFilePlain, false)
	if err != nil {
		return nil, err
	}
	compressed, err := d.statVariant(repo, tagsFileGzip, true)
	if err != nil {
		return nil, err
	}

	switch {
	case !plain.exists && !compressed.exists:
		return []string{}, nil
	case plain.exists && compressed.exists:
		primary, fallback := newerVariant(plain, compressed)
		tags, usedFallback, err := readTagsVariant(primary, fallback)
		if err != nil {
			return nil, err
		}
		if !usedFallback {
			removeStaleVariant(fallback)
		}
		return tags, nil
	case plain.exists:
		tags, _, err := readTagsVariant(plain, compressed)
		return tags, err
	default:
		tags, _, err := readTagsVariant(compressed, plain)
		return tags, err
	}
}

// SetTags implements the DatabaseWriter interface, recording tags for the repo.
func (d *FilesystemDatabase) SetTags(ctx context.Context, repo RepoIdentity, tags []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateRepoIdentity(repo); err != nil {
		return "", err
	}

	serialized := marshalTagsLines(tags)
	sum := sha256.Sum256(serialized)
	revision := fmt.Sprintf("sha256:%x", sum)

	filename := tagsFilePlain
	payload := serialized
	if len(serialized) >= d.compressionThreshold {
		filename = tagsFileGzip
		compressed, err := gzipTags(serialized)
		if err != nil {
			return "", err
		}
		payload = compressed
	}

	artifact := artifactForRepo(repo, filename)
	if err := d.storage.MkdirAll(artifact); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := d.storage.AtomicWriteFile(&artifact, bytes.NewReader(payload), 0o600); err != nil {
		return "", fmt.Errorf("failed to write tags: %w", err)
	}

	if filename == tagsFilePlain {
		removeStaleVariant(tagFileVariant{path: d.storage.LocalPath(artifactForRepo(repo, tagsFileGzip))})
	} else {
		removeStaleVariant(tagFileVariant{path: d.storage.LocalPath(artifactForRepo(repo, tagsFilePlain))})
	}

	return revision, nil
}

// Delete implements the DatabaseWriter interface, deleting tags for the repo.
func (d *FilesystemDatabase) Delete(ctx context.Context, repo RepoIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateRepoIdentity(repo); err != nil {
		return err
	}
	_, err := d.storage.RemoveAll(artifactForRepo(repo, tagsFilePlain))
	if err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}
	return nil
}

func validateRepoIdentity(repo RepoIdentity) error {
	if repo.Namespace == "" || repo.Name == "" {
		return fmt.Errorf("repo namespace and name are required")
	}
	return nil
}

func artifactForRepo(repo RepoIdentity, filename string) meta.Artifact {
	return meta.Artifact{
		Path: artifactstorage.ArtifactPath(imagev1.ImageRepositoryKind, repo.Namespace, repo.Name, filename),
	}
}

func marshalTagsLines(tags []string) []byte {
	var b bytes.Buffer
	for _, tag := range tags {
		b.WriteString(tag)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func gzipTags(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("failed to compress tags: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}
	return b.Bytes(), nil
}

type tagFileVariant struct {
	path       string
	compressed bool
	exists     bool
	info       os.FileInfo
}

func (d *FilesystemDatabase) statVariant(repo RepoIdentity, filename string, compressed bool) (tagFileVariant, error) {
	artifact := artifactForRepo(repo, filename)
	variant := tagFileVariant{
		path:       d.storage.LocalPath(artifact),
		compressed: compressed,
	}
	info, err := os.Stat(variant.path)
	if errors.Is(err, os.ErrNotExist) {
		return variant, nil
	}
	if err != nil {
		return variant, fmt.Errorf("failed to stat %s: %w", variant.path, err)
	}
	variant.exists = true
	variant.info = info
	return variant, nil
}

func newerVariant(a, b tagFileVariant) (tagFileVariant, tagFileVariant) {
	if a.info.ModTime().After(b.info.ModTime()) {
		return a, b
	}
	return b, a
}

func readTagsVariant(primary, fallback tagFileVariant) ([]string, bool, error) {
	tags, err := readTagFile(primary)
	if errors.Is(err, os.ErrNotExist) && fallback.path != "" {
		tags, fallbackErr := readTagFile(fallback)
		if fallbackErr == nil {
			return tags, true, nil
		}
		if errors.Is(fallbackErr, os.ErrNotExist) {
			return []string{}, false, nil
		}
		return nil, false, fallbackErr
	}
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, false, nil
	}
	return tags, false, err
}

func readTagFile(variant tagFileVariant) (_ []string, retErr error) {
	file, err := os.Open(variant.path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	var reader io.Reader = file
	var gzipReader *gzip.Reader
	if variant.compressed {
		gzipReader, err = gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to open gzip tags: %w", err)
		}
		defer func() {
			if err := gzipReader.Close(); err != nil && retErr == nil {
				retErr = err
			}
		}()
		reader = gzipReader
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256), 1024)
	tags := []string{}
	for scanner.Scan() {
		tags = append(tags, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read tags: %w", err)
	}
	return tags, nil
}

func removeStaleVariant(variant tagFileVariant) {
	if variant.path == "" {
		return
	}
	if err := os.Remove(variant.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
}
