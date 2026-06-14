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

import "context"

// RepoIdentity identifies an ImageRepository for tag storage. Implementations
// choose which fields form their storage key:
//   - BadgerDatabase keys by CanonicalName.
//   - FilesystemDatabase keys by Namespace and Name.
type RepoIdentity struct {
	Namespace     string
	Name          string
	CanonicalName string
}

// Database combines tag read and write operations.
type Database interface {
	DatabaseWriter
	DatabaseReader
}

// DatabaseWriter implementations record the tags for an image repository.
type DatabaseWriter interface {
	SetTags(ctx context.Context, repo RepoIdentity, tags []string) (revision string, err error)
	Delete(ctx context.Context, repo RepoIdentity) error
}

// DatabaseReader implementations get the stored set of tags for an image
// repository.
//
// If no tags are available for the repo, then implementations should return an
// empty set of tags.
type DatabaseReader interface {
	Tags(ctx context.Context, repo RepoIdentity) (tags []string, err error)
}
