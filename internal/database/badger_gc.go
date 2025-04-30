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
	"errors"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/go-logr/logr"
)

type BadgerGarbageCollector struct {
	// settings
	DiscardRatio float64
	Interval     time.Duration
	// external deps
	db  *badger.DB
	log *logr.Logger
	// flow control
	timer   *time.Timer
	running sync.Mutex
}

// NewBadgerGarbageCollector creates and returns a new
func NewBadgerGarbageCollector(db *badger.DB, interval time.Duration, log *logr.Logger) *BadgerGarbageCollector {
	return &BadgerGarbageCollector{
		DiscardRatio: 0.5, // must be a float between 0.0 and 1.0, inclusive
		Interval:     interval,

		db:  db,
		log: log,
	}
}

// Start repeatedly runs the BadgerDB garbage collector with a delay inbetween
// runs.
//
// This is a non-blocking operation.
// To stop the garbage collector, call Stop().
func (gc *BadgerGarbageCollector) Start() {
	gc.log.Info("Starting Badger GC")
	gc.timer = time.AfterFunc(gc.Interval, func() {
		gc.running.Lock()
		gc.discardValueLogFiles()
		gc.running.Unlock()
		gc.timer.Reset(gc.Interval)
	})
}

// Stop blocks until the garbage collector has been stopped.
//
// To avoid GC Errors, call Stop() before closing the database.
func (gc *BadgerGarbageCollector) Stop() {
	gc.log.Info("Sending stop to Badger GC")
	gc.timer.Stop()
	gc.running.Lock()
	gc.running.Unlock()
	gc.log.Info("Stopped Badger GC")
}

// upper bound for loop
const maxDiscards = 1000

func (gc *BadgerGarbageCollector) discardValueLogFiles() {
	for c := 0; c < maxDiscards; c++ {
		err := gc.db.RunValueLogGC(gc.DiscardRatio)
		if errors.Is(err, badger.ErrNoRewrite) {
			// there is no more garbage to discard
			gc.log.Info("Ran Badger GC", "discarded_vlogs", c)
			return
		}
		if err != nil {
			gc.log.Error(err, "Badger GC Error", "discarded_vlogs", c)
			return
		}
	}
	gc.log.Info("Ran Badger GC for maximum discards", "discarded_vlogs", maxDiscards)
}
