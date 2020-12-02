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
	"reflect"
	"sort"
	"testing"
)

func TestRegexFilter(t *testing.T) {
	cases := []struct {
		label    string
		tags     []string
		pattern  string
		extract  string
		expected []string
	}{
		{
			label:    "none",
			tags:     []string{"a"},
			expected: []string{"a"},
		},
		{
			label:    "valid pattern",
			tags:     []string{"ver1", "ver2", "ver3", "rel1"},
			pattern:  "^ver",
			expected: []string{"ver1", "ver2", "ver3"},
		},
		{
			label:    "valid pattern with capture group",
			tags:     []string{"ver1", "ver2", "ver3", "rel1"},
			pattern:  `ver(\d+)`,
			extract:  `$1`,
			expected: []string{"1", "2", "3"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.label, func(t *testing.T) {
			filter := newRegexFilter(tt.pattern, tt.extract)
			filter.Apply(tt.tags)
			r := sort.StringSlice(filter.Items())
			if reflect.DeepEqual(r, tt.expected) {
				t.Errorf("incorrect value returned, got '%s', expected '%s'", r, tt.expected)
			}
		})
	}
}

func newRegexFilter(pattern string, extract string) *RegexFilter {
	f, _ := NewRegexFilter(pattern, extract)
	return f
}
