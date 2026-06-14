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
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestReconcileFormat(t *testing.T) {
	tests := []struct {
		name              string
		filesystemEnabled bool
		version           *string
		data              bool
		wantEntries       []string
	}{
		{
			name:              "badger format remains untouched",
			filesystemEnabled: false,
			data:              true,
			wantEntries:       []string{"data"},
		},
		{
			name:              "enable filesystem storage wipes old data",
			filesystemEnabled: true,
			data:              true,
			wantEntries:       []string{storageVersionFile},
		},
		{
			name:              "disable filesystem storage wipes old data",
			filesystemEnabled: false,
			version:           new(filesystemStorageVersion),
			data:              true,
			wantEntries:       []string{},
		},
		{
			name:              "filesystem format remains untouched",
			filesystemEnabled: true,
			version:           new(filesystemStorageVersion),
			data:              true,
			wantEntries:       []string{storageVersionFile, "data"},
		},
		{
			name:              "unknown version is replaced",
			filesystemEnabled: true,
			version:           new("unknown"),
			data:              true,
			wantEntries:       []string{storageVersionFile},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.version != nil {
				if err := os.WriteFile(filepath.Join(dir, storageVersionFile), []byte(*tt.version), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tt.data {
				if err := os.WriteFile(filepath.Join(dir, "data"), []byte("data"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := ReconcileFormat(dir, tt.filesystemEnabled); err != nil {
				t.Fatal(err)
			}

			entries := readEntryNames(t, dir)
			if !reflect.DeepEqual(tt.wantEntries, entries) {
				t.Fatalf("entries got %#v, want %#v", entries, tt.wantEntries)
			}
			if contains(entries, storageWipeInProgressFile) {
				t.Fatal("wipe marker was not removed")
			}
		})
	}
}

func TestReconcileFormatCompletesInterruptedWipe(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, storageVersionFile), []byte(filesystemStorageVersion), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, storageWipeInProgressFile), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "leftover"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ReconcileFormat(dir, true); err != nil {
		t.Fatal(err)
	}

	entries := readEntryNames(t, dir)
	want := []string{storageVersionFile}
	if !reflect.DeepEqual(want, entries) {
		t.Fatalf("entries got %#v, want %#v", entries, want)
	}
}

func readEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func contains(items []string, item string) bool {
	for _, v := range items {
		if v == item {
			return true
		}
	}
	return false
}
