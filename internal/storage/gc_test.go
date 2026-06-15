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
	"os"
	"path/filepath"
	"testing"
	"time"

	artifactstorage "github.com/fluxcd/pkg/artifact/storage"
	"github.com/go-logr/logr/testr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
)

func TestFilesystemGarbageCollectorNeedLeaderElection(t *testing.T) {
	gc := &FilesystemGarbageCollector{}
	if !gc.NeedLeaderElection() {
		t.Fatal("NeedLeaderElection() returned false")
	}
}

func TestFilesystemGarbageCollectorDeletesOrphans(t *testing.T) {
	st, cl := newGCTestStorage(t, &imagev1.ImageRepository{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "keep"}})
	createRepoDir(t, st, "default", "keep")
	createRepoDir(t, st, "default", "orphan")

	gc := NewFilesystemGarbageCollector("test-gc", st, cl, time.Minute)
	gc.log = testr.New(t)
	gc.collect(context.Background())

	if !repoDirExists(t, st, "default", "keep") {
		t.Fatal("responsible repo was deleted")
	}
	if repoDirExists(t, st, "default", "orphan") {
		t.Fatal("orphan repo was not deleted")
	}
}

func TestFilesystemGarbageCollectorSkipsDeleteOnListError(t *testing.T) {
	st, cl := newGCTestStorage(t)
	createRepoDir(t, st, "default", "orphan")

	gc := NewFilesystemGarbageCollector("test-gc", st, listErrorClient{Client: cl}, time.Minute)
	gc.log = testr.New(t)
	gc.collect(context.Background())

	if !repoDirExists(t, st, "default", "orphan") {
		t.Fatal("orphan repo was deleted after list error")
	}
}

func TestFilesystemGarbageCollectorEmptyResponsibleSetDeletesAll(t *testing.T) {
	st, cl := newGCTestStorage(t)
	createRepoDir(t, st, "default", "one")
	createRepoDir(t, st, "default", "two")

	gc := NewFilesystemGarbageCollector("test-gc", st, cl, time.Minute)
	gc.log = testr.New(t)
	gc.collect(context.Background())

	if repoDirExists(t, st, "default", "one") || repoDirExists(t, st, "default", "two") {
		t.Fatal("repos were not deleted for empty responsible set")
	}
}

func TestFilesystemGarbageCollectorPrunesEmptyNamespaceDirs(t *testing.T) {
	st, cl := newGCTestStorage(t, &imagev1.ImageRepository{ObjectMeta: metav1.ObjectMeta{Namespace: "keep-ns", Name: "keep"}})
	createRepoDir(t, st, "keep-ns", "keep")
	// orphan repo whose namespace becomes empty once it's pruned
	createRepoDir(t, st, "orphan-ns", "orphan")

	gc := NewFilesystemGarbageCollector("test-gc", st, cl, time.Minute)
	gc.log = testr.New(t)
	gc.collect(context.Background())

	if namespaceDirExists(t, st, "orphan-ns") {
		t.Fatal("empty namespace directory was not pruned")
	}
	if !namespaceDirExists(t, st, "keep-ns") {
		t.Fatal("namespace directory with a live repo was pruned")
	}
}

type listErrorClient struct {
	client.Client
}

func (c listErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return errors.New("list failed")
}

func newGCTestStorage(t *testing.T, objs ...client.Object) (*artifactstorage.Storage, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := imagev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return &artifactstorage.Storage{BasePath: t.TempDir()}, fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func createRepoDir(t *testing.T, st *artifactstorage.Storage, namespace, name string) {
	t.Helper()
	path := filepath.Join(st.BasePath, "imagerepository", namespace, name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func namespaceDirExists(t *testing.T, st *artifactstorage.Storage, namespace string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(st.BasePath, "imagerepository", namespace))
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatal(err)
	return false
}

func repoDirExists(t *testing.T, st *artifactstorage.Storage, namespace, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(st.BasePath, "imagerepository", namespace, name))
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatal(err)
	return false
}
