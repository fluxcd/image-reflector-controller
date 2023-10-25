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
	"sort"
	"testing"

	. "github.com/onsi/gomega"
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
		{
			label: "valid pattern (complex regex 1)",
			tags: []string{
				"123-123.123.abcd123-debug",
				"123-123.123.abcd123",
				"123-123.123.abcd456-debug",
				"123-123.123.abcd456",
			},
			pattern: `^(123-[0-9]+\.[0-9]+\.[a-z0-9]+-debug)`,
			expected: []string{
				"123-123.123.abcd123-debug",
				"123-123.123.abcd456-debug",
			},
		},
		{
			label: "valid pattern with capture group (complex regex 2)",
			tags: []string{
				"123-123.123.abcd123-debug",
				"123-123.123.abcd123",
				"123-123.123.abcd456-debug",
				"123-123.123.abcd456",
			},
			pattern: `^(?P<tag>123-[0-9]+\.[0-9]+\.[a-z0-9]+[^-debug])`,
			extract: `$tag`,
			expected: []string{
				"123-123.123.abcd123",
				"123-123.123.abcd456",
			},
		},
		{
			label: "valid pattern with capture group (complex regex 3)",
			tags: []string{
				"123-123.123.abcd123-debug",
				"123-123.123.abcd123",
				"123-123.123.abcd456-debug",
				"123-123.123.abcd456",
			},
			pattern: `^(?P<tag>123-[0-9]+\.[0-9]+\.[a-z0-9]+$)`,
			extract: `$tag`,
			expected: []string{
				"123-123.123.abcd123",
				"123-123.123.abcd456",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.label, func(t *testing.T) {
			g := NewWithT(t)

			f, err := NewRegexFilter(tt.pattern, tt.extract)
			g.Expect(err).ToNot(HaveOccurred())

			f.Apply(tt.tags)
			r := f.Items()
			sort.Strings(r)

			g.Expect(r).To(Equal(tt.expected))
		})
	}
}
