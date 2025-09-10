/*
Copyright 2025 The Flux authors

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
	"errors"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// BadgerGarbageCollector implements controller runtime's Runnable
type BadgerGarbageCollector struct {
	// DiscardRatio must be a float between 0.0 and 1.0, inclusive
	// See badger.DB.RunValueLogGC for more info
	DiscardRatio float64
	Interval     time.Duration

	name string
	db   *badger.DB
	log  logr.Logger
}

// NewBadgerGarbageCollector creates and returns a new BadgerGarbageCollector
func NewBadgerGarbageCollector(name string, db *badger.DB, interval time.Duration, discardRatio float64) *BadgerGarbageCollector {
	return &BadgerGarbageCollector{
		DiscardRatio: discardRatio,
		Interval:     interval,

		name: name,
		db:   db,
	}
}

// Start repeatedly runs the BadgerDB garbage collector with a delay inbetween
// runs.
//
// Start blocks until the context is cancelled. The database is expected to
// already be open and not be closed while this context is active.
//
// ctx should be a logr.Logger context.
func (gc *BadgerGarbageCollector) Start(ctx context.Context) error {
	gc.log = ctrl.LoggerFrom(ctx).WithName(gc.name)

	gc.log.Info("Starting Badger GC")
	timer := time.NewTimer(gc.Interval)
	for {
		select {
		case <-timer.C:
			gc.discardValueLogFiles()
			timer.Reset(gc.Interval)
		case <-ctx.Done():
			timer.Stop()
			gc.log.Info("Stopped Badger GC")
			return nil
		}
	}
}

// upper bound for loop
const maxDiscards = 1000

func (gc *BadgerGarbageCollector) discardValueLogFiles() {
	gc.log.V(1).Info("Running Badger GC")
	for c := 0; c < maxDiscards; c++ {
		err := gc.db.RunValueLogGC(gc.DiscardRatio)
		if errors.Is(err, badger.ErrNoRewrite) {
			// there is no more garbage to discard
			gc.log.V(1).Info("Ran Badger GC", "discarded_vlogs", c)
			return
		}
		if err != nil {
			gc.log.Error(err, "Badger GC Error", "discarded_vlogs", c)
			return
		}
	}
	gc.log.Error(nil, "Warning: Badger GC ran for maximum discards", "discarded_vlogs", maxDiscards)
}
