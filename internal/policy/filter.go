/*
Copyright 2021 The Flux authors

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

package policy

import (
	"fmt"
	"regexp"
)

// RegexFilter represents a regular expression filter
type RegexFilter struct {
	filtered map[string]string

	Regexp  *regexp.Regexp
	Replace string
}

// NewRegexFilter constructs new RegexFilter object
func NewRegexFilter(pattern string, replace string) (*RegexFilter, error) {
	m, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression pattern '%s': %w", pattern, err)
	}
	return &RegexFilter{
		Regexp:  m,
		Replace: replace,
	}, nil
}

// Apply will construct the filtered list of tags based on the provided list of tags
func (f *RegexFilter) Apply(list []string) {
	f.filtered = map[string]string{}
	for _, item := range list {
		if submatches := f.Regexp.FindStringSubmatchIndex(item); len(submatches) > 0 {
			tag := item
			if f.Replace != "" {
				result := []byte{}
				result = f.Regexp.ExpandString(result, f.Replace, item, submatches)
				tag = string(result)
			}
			f.filtered[tag] = item
		}
	}
}

// Items returns the list of filtered tags
func (f *RegexFilter) Items() []string {
	var filtered []string
	for k := range f.filtered {
		filtered = append(filtered, k)
	}
	return filtered
}

// GetOriginalTag returns the original tag before replace extraction
func (f *RegexFilter) GetOriginalTag(tag string) string {
	return f.filtered[tag]
}
