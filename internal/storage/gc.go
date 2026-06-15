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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	artifactstorage "github.com/fluxcd/pkg/artifact/storage"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
)

// FilesystemGarbageCollector removes tag files for ImageRepository objects no
// longer handled by this controller instance.
type FilesystemGarbageCollector struct {
	Interval time.Duration

	name    string
	storage *artifactstorage.Storage
	client  client.Client
	log     logr.Logger
}

// NewFilesystemGarbageCollector creates a FilesystemGarbageCollector.
func NewFilesystemGarbageCollector(name string, storage *artifactstorage.Storage, client client.Client, interval time.Duration) *FilesystemGarbageCollector {
	return &FilesystemGarbageCollector{
		Interval: interval,
		name:     name,
		storage:  storage,
		client:   client,
	}
}

// NeedLeaderElection ensures the GC runs only on the elected leader.
func (gc *FilesystemGarbageCollector) NeedLeaderElection() bool {
	return true
}

// Start runs the filesystem storage garbage collector after each interval.
func (gc *FilesystemGarbageCollector) Start(ctx context.Context) error {
	gc.log = ctrl.LoggerFrom(ctx).WithName(gc.name)
	gc.log.Info("Starting filesystem storage GC")

	timer := time.NewTimer(gc.Interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			gc.collect(ctx)
			timer.Reset(gc.Interval)
		case <-ctx.Done():
			gc.log.Info("Stopped filesystem storage GC")
			return nil
		}
	}
}

func (gc *FilesystemGarbageCollector) collect(ctx context.Context) {
	gc.log.V(1).Info("Running filesystem storage GC")
	onAPI, err := gc.repositoriesOnTheAPI(ctx)
	if err != nil {
		gc.log.Error(err, "failed to list ImageRepositories for filesystem storage GC")
		return
	}

	onDisk, err := gc.repositoriesOnDisk()
	if err != nil {
		gc.log.Error(err, "failed to enumerate filesystem storage entries")
		return
	}

	deleted := 0
	for _, repo := range onDisk {
		if _, ok := onAPI[repo]; ok {
			continue
		}
		artifact := meta.Artifact{Path: artifactstorage.ArtifactPath(imagev1.ImageRepositoryKind, repo.Namespace, repo.Name, tagsFilePlain)}
		deletedDir, err := gc.storage.RemoveAll(artifact)
		if err != nil {
			gc.log.Error(err, "failed to delete orphaned filesystem storage entry", "repository", repo.String())
			continue
		}
		deleted++
		gc.log.V(1).Info("deleted orphaned filesystem storage entry", "repository", repo.String(), "path", deletedDir)
	}

	if err := gc.pruneEmptyNamespaceDirs(); err != nil {
		gc.log.Error(err, "failed to prune empty namespace directories")
	}

	gc.log.V(1).Info("Ran filesystem storage GC", "deleted_entries", deleted)
}

// pruneEmptyNamespaceDirs removes namespace directories left empty after their
// last repository entry is deleted, so empty directories don't accumulate.
func (gc *FilesystemGarbageCollector) pruneEmptyNamespaceDirs() error {
	base := filepath.Join(gc.storage.BasePath, strings.ToLower(imagev1.ImageRepositoryKind))
	namespaces, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", base, err)
	}

	for _, namespace := range namespaces {
		if !namespace.IsDir() {
			continue
		}
		nsPath := filepath.Join(base, namespace.Name())
		entries, err := os.ReadDir(nsPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", nsPath, err)
		}
		if len(entries) > 0 {
			continue
		}
		if err := os.Remove(nsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove empty namespace directory %s: %w", nsPath, err)
		}
		gc.log.V(1).Info("removed empty namespace directory", "path", nsPath)
	}
	return nil
}

func (gc *FilesystemGarbageCollector) repositoriesOnTheAPI(ctx context.Context) (map[client.ObjectKey]struct{}, error) {
	var repos imagev1.ImageRepositoryList
	if err := gc.client.List(ctx, &repos); err != nil {
		return nil, err
	}

	result := make(map[client.ObjectKey]struct{}, len(repos.Items))
	for i := range repos.Items {
		repo := client.ObjectKey{Namespace: repos.Items[i].Namespace, Name: repos.Items[i].Name}
		result[repo] = struct{}{}
	}
	return result, nil
}

func (gc *FilesystemGarbageCollector) repositoriesOnDisk() ([]client.ObjectKey, error) {
	base := filepath.Join(gc.storage.BasePath, strings.ToLower(imagev1.ImageRepositoryKind))
	namespaces, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", base, err)
	}

	var result []client.ObjectKey
	for _, namespace := range namespaces {
		if !namespace.IsDir() {
			continue
		}
		nsPath := filepath.Join(base, namespace.Name())
		repos, err := os.ReadDir(nsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", nsPath, err)
		}
		for _, repo := range repos {
			if !repo.IsDir() {
				continue
			}
			result = append(result, client.ObjectKey{Namespace: namespace.Name(), Name: repo.Name()})
		}
	}
	return result, nil
}
