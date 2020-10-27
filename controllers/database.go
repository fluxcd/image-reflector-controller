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

package controllers

import (
	"sync"
)

type database struct {
	mu       sync.RWMutex
	repoTags map[string][]string
}

func NewDatabase() *database {
	return &database{
		repoTags: map[string][]string{},
	}
}

func (db *database) Tags(repo string) []string {
	db.mu.RLock()
	tags := db.repoTags[repo]
	db.mu.RUnlock()
	return tags
}

func (db *database) SetTags(repo string, tags []string) {
	db.mu.Lock()
	db.repoTags[repo] = tags
	db.mu.Unlock()
}
