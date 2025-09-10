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

package controller

// DatabaseWriter implementations record the tags for an image repository.
type DatabaseWriter interface {
	SetTags(repo string, tags []string) (string, error)
}

// DatabaseReader implementations get the stored set of tags for an image
// repository.
//
// If no tags are availble for the repo, then implementations should return an
// empty set of tags.
type DatabaseReader interface {
	Tags(repo string) ([]string, error)
}
