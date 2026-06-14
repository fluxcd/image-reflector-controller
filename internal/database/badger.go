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
	"encoding/json"
	"fmt"
	"hash/adler32"

	"github.com/dgraph-io/badger/v4"

	"github.com/fluxcd/image-reflector-controller/internal/storage"
)

const tagsPrefix = "tags"

// BadgerDatabase provides implementations of the tags database based on Badger.
type BadgerDatabase struct {
	db *badger.DB
}

// NewBadgerDatabase creates and returns a new database implementation using
// Badger for storing the image tags.
func NewBadgerDatabase(db *badger.DB) *BadgerDatabase {
	return &BadgerDatabase{
		db: db,
	}
}

// Tags implements the DatabaseReader interface, fetching the tags for the repo.
//
// If the repo does not exist, an empty set of tags is returned.
func (a *BadgerDatabase) Tags(ctx context.Context, repo storage.RepoIdentity) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var tags []string
	err := a.db.View(func(txn *badger.Txn) error {
		var err error
		tags, err = getOrEmpty(txn, repo.CanonicalName)
		return err
	})
	return tags, err
}

// SetTags implements the DatabaseWriter interface, recording the tags against
// the repo.
//
// It overwrites existing tag sets for the provided repo.
func (a *BadgerDatabase) SetTags(ctx context.Context, repo storage.RepoIdentity, tags []string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	b, err := marshal(tags)
	if err != nil {
		return "", err
	}
	err = a.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry(keyForRepo(tagsPrefix, repo.CanonicalName), b)
		return txn.SetEntry(e)
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", adler32.Checksum(b)), nil
}

// Delete implements the DatabaseWriter interface. Badger keys tags by canonical
// image name, which may be shared by multiple ImageRepository objects, so delete
// is intentionally a no-op to preserve existing behavior.
func (a *BadgerDatabase) Delete(ctx context.Context, repo storage.RepoIdentity) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func keyForRepo(prefix, repo string) []byte {
	return []byte(fmt.Sprintf("%s:%s", prefix, repo))
}

func getOrEmpty(txn *badger.Txn, repo string) ([]string, error) {
	item, err := txn.Get(keyForRepo(tagsPrefix, repo))
	if err == badger.ErrKeyNotFound {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var tags []string
	err = item.Value(func(val []byte) error {
		tags, err = unmarshal(val)
		return err
	})
	return tags, err
}

func marshal(t []string) ([]byte, error) {
	return json.Marshal(t)
}

func unmarshal(b []byte) ([]string, error) {
	var tags []string
	if err := json.Unmarshal(b, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}
