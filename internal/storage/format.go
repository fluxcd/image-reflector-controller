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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	storageVersionFile        = ".storage-version"
	storageWipeInProgressFile = ".storage-wipe-in-progress"
	filesystemStorageVersion  = "2"
)

// ReconcileFormat prepares the storage root for the selected storage backend.
// Switching backends wipes the rebuildable tag cache before either backend is
// initialized.
func ReconcileFormat(storagePath string, filesystemStorageEnabled bool) error {
	if err := os.MkdirAll(storagePath, 0o700); err != nil {
		return fmt.Errorf("failed to create storage path: %w", err)
	}

	version, versionExists, err := readStorageVersion(storagePath)
	if err != nil {
		return err
	}

	if !storageFormatMatches(version, versionExists, filesystemStorageEnabled) {
		if err := ensureWipeMarker(storagePath); err != nil {
			return err
		}
		if err := setStorageVersion(storagePath, filesystemStorageEnabled); err != nil {
			return err
		}
	}

	wipeMarkerExists, err := pathExists(filepath.Join(storagePath, storageWipeInProgressFile))
	if err != nil {
		return err
	}
	if !wipeMarkerExists {
		return nil
	}

	if err := wipeStoragePath(storagePath); err != nil {
		return err
	}
	if err := syncDir(storagePath); err != nil {
		return fmt.Errorf("failed to sync storage path after wipe: %w", err)
	}
	if err := os.Remove(filepath.Join(storagePath, storageWipeInProgressFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove wipe marker: %w", err)
	}
	if err := syncDir(storagePath); err != nil {
		return fmt.Errorf("failed to sync storage path after marker removal: %w", err)
	}

	return nil
}

func readStorageVersion(storagePath string) (string, bool, error) {
	b, err := os.ReadFile(filepath.Join(storagePath, storageVersionFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("failed to read storage version: %w", err)
	}
	return strings.TrimSpace(string(b)), true, nil
}

func storageFormatMatches(version string, versionExists, filesystemStorageEnabled bool) bool {
	if filesystemStorageEnabled {
		return versionExists && version == filesystemStorageVersion
	}
	return !versionExists
}

func ensureWipeMarker(storagePath string) error {
	path := filepath.Join(storagePath, storageWipeInProgressFile)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create wipe marker: %w", err)
	}
	if err := syncFileAndClose(file); err != nil {
		return fmt.Errorf("failed to sync wipe marker: %w", err)
	}
	if err := syncDir(storagePath); err != nil {
		return fmt.Errorf("failed to sync storage path after marker creation: %w", err)
	}
	return nil
}

func setStorageVersion(storagePath string, filesystemStorageEnabled bool) error {
	path := filepath.Join(storagePath, storageVersionFile)
	if !filesystemStorageEnabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove storage version: %w", err)
		}
		if err := syncDir(storagePath); err != nil {
			return fmt.Errorf("failed to sync storage path after version removal: %w", err)
		}
		return nil
	}

	if err := writeFileSync(storagePath, storageVersionFile, []byte(filesystemStorageVersion+"\n"), 0o600); err != nil {
		return fmt.Errorf("failed to write storage version: %w", err)
	}
	return nil
}

func writeFileSync(dir, filename string, data []byte, mode os.FileMode) (retErr error) {
	tmp, err := os.CreateTemp(dir, filename+"-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if retErr != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := syncFileAndClose(tmp); err != nil {
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, filename)); err != nil {
		return err
	}
	return syncDir(dir)
}

func wipeStoragePath(storagePath string) error {
	entries, err := os.ReadDir(storagePath)
	if err != nil {
		return fmt.Errorf("failed to read storage path: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == storageVersionFile || name == storageWipeInProgressFile {
			continue
		}
		if err := os.RemoveAll(filepath.Join(storagePath, name)); err != nil {
			return fmt.Errorf("failed to remove storage entry %q: %w", name, err)
		}
	}
	return nil
}

func syncFileAndClose(file *os.File) error {
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func syncDir(path string) (retErr error) {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := dir.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()
	return dir.Sync()
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("failed to stat %s: %w", path, err)
}
