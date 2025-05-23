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
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/pkg/errors"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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

func TestBadgerGCLoad(t *testing.T) {
	badger, db := createBadgerDatabaseForGC(t)
	ctx, cancel := context.WithCancel(
		logr.NewContext(context.Background(), testr.NewWithOptions(t, testr.Options{Verbosity: 1, LogTimestamp: true})))

	stop := make(chan struct{})
	go func() {
		gc := NewBadgerGarbageCollector("loaded-badger-gc", badger, time.Second*20, 0.7)
		gc.Start(ctx)
		stop <- struct{}{}
	}()

	repos := []string{"alpine", "node", "postgres", "debian"}
	for i := 5; i >= 0; i-- {
		for _, repo := range repos {
			ref, err := name.ParseReference(repo)
			fatalIfError(t, err)
			tags, err := remote.List(ref.Context())
			iter := (i + 3) * 15000
			for r := 0; r <= iter; r++ {
				fatalIfError(t, err)
				db.SetTags(fmt.Sprintf("%s-%d", repo, r), tags[0:len(tags)-i])
				// time.Sleep(time.Millisecond)
			}
			t.Logf("%s %d: %d repos", repo, i, iter)
		}
		time.Sleep(time.Millisecond * 100)
	}

	cancel()
	t.Log("waiting for GC stop")
	select {
	case <-time.NewTimer(30 * time.Second).C:
		t.Fatalf("GC did not stop")
	case <-stop:
		t.Log("GC Stopped")
	}
}

type badgerTestLogger struct {
	logger logr.Logger
}

func (l *badgerTestLogger) Errorf(f string, v ...interface{}) {
	l.logger.Error(errors.Errorf("ERROR: "+f, v...), f)
}
func (l *badgerTestLogger) Infof(f string, v ...interface{}) {
	l.log("INFO", f, v...)
}
func (l *badgerTestLogger) Warningf(f string, v ...interface{}) {
	l.log("WARNING", f, v...)
}
func (l *badgerTestLogger) Debugf(f string, v ...interface{}) {
	l.log("DEBUG", f, v...)
}

var filter = regexp.MustCompile(`writeRequests called. Writing to value log|2 entries written|Writing to memtable|Sending updates to subscribers|Found value log max|fid:|Moved: 0|Processed 0 entries in 0 loops|Discard stats: map`)

func (l *badgerTestLogger) log(lvl string, f string, v ...interface{}) {
	str := fmt.Sprintf(lvl+": "+f, v...)
	if filter.MatchString(str) {
		return
	}
	l.logger.Info(str)
}

func createBadgerDatabaseForGC(t *testing.T) (*badger.DB, *BadgerDatabase) {
	t.Helper()
	dir, err := os.MkdirTemp(os.TempDir(), t.Name())
	if err != nil {
		t.Fatal(err)
	}
	opts := badger.DefaultOptions(dir)
	opts = opts.WithValueThreshold(100)      // force values into the vlog files
	opts = opts.WithValueLogMaxEntries(1000) // force many vlogs to be created
	opts = opts.WithValueLogFileSize(32 << 19)
	opts = opts.WithMemTableSize(16 << 19) // fill up memtables quickly
	opts = opts.WithNumMemtables(1)
	opts = opts.WithNumLevelZeroTables(1) // hold fewer memtables in memory
	opts = opts.WithLogger(&badgerTestLogger{logger: testr.NewWithOptions(t, testr.Options{LogTimestamp: true})})
	// opts = opts.WithLoggingLevel(badger.DEBUG)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return db, NewBadgerDatabase(db)
}
