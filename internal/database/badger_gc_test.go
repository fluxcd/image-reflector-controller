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
	"context"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
)

func TestBadgerGarbageCollectorDoesStop(t *testing.T) {
	badger, db := createBadgerDatabaseForGC(t)
	ctx, cancel := context.WithCancel(
		logr.NewContext(context.Background(),
			testr.NewWithOptions(t, testr.Options{Verbosity: 1, LogTimestamp: true})))

	stop := make(chan struct{})
	go func() {
		gc := NewBadgerGarbageCollector("test-badger-gc", badger, 500*time.Millisecond, 0.01)
		gc.Start(ctx)
		stop <- struct{}{}
	}()

	time.Sleep(time.Second)

	tags := []string{"latest", "v0.0.1", "v0.0.2"}
	_, err := db.SetTags(testRepo, tags)
	fatalIfError(t, err)
	_, err = db.Tags(testRepo)
	fatalIfError(t, err)
	t.Log("wrote tags successfully")

	time.Sleep(time.Second)

	cancel()
	t.Log("waiting for GC stop")
	select {
	case <-time.NewTimer(5 * time.Second).C:
		t.Fatalf("GC did not stop")
	case <-stop:
		t.Log("GC Stopped")
	}
}

func createBadgerDatabaseForGC(t *testing.T) (*badger.DB, *BadgerDatabase) {
	t.Helper()
	dir, err := os.MkdirTemp(os.TempDir(), t.Name())
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
	return db, NewBadgerDatabase(db)
}
