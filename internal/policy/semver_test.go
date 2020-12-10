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

package policy

import (
	"testing"
)

func TestNewSemVer(t *testing.T) {
	cases := []struct {
		label        string
		semverRanges []string
		expectErr    bool
	}{
		{
			label:        "With valid range",
			semverRanges: []string{"1.0.x", "^1.0", "=1.0.0", "~1.0", ">=1.0", ">0,<2.0"},
		},
		{
			label:        "With invalid range",
			semverRanges: []string{"1.0.0p", "1x", "x1", "-1", "a", ""},
			expectErr:    true,
		},
	}

	for _, tt := range cases {
		for _, r := range tt.semverRanges {
			t.Run(tt.label, func(t *testing.T) {
				_, err := NewSemVer(r)
				if tt.expectErr && err == nil {
					t.Fatalf("expecting error, got nil for range value: '%s'", r)
				}
				if !tt.expectErr && err != nil {
					t.Fatalf("returned unexpected error: %s", err)
				}
			})
		}
	}
}

func TestSemVer_Latest(t *testing.T) {
	cases := []struct {
		label           string
		semverRange     string
		versions        []string
		expectedVersion string
		expectErr       bool
	}{
		{
			label:           "Regular",
			versions:        []string{"1.0.0", "1.0.0.1", "1.0.0p", "1.0.1", "1.2.0", "0.1.0"},
			semverRange:     "1.0.x",
			expectedVersion: "1.0.1",
		},
		{
			label:           "Regular with prefix",
			versions:        []string{"v1.2.3", "v1.0.0", "v0.1.0"},
			semverRange:     "1.0.x",
			expectedVersion: "v1.0.0",
		},
		{
			label:       "With invalid prefix",
			versions:    []string{"b1.2.3", "b1.0.0", "b0.1.0"},
			semverRange: "1.0.x",
			expectErr:   true,
		},
		{
			label:       "With empty list",
			versions:    []string{},
			semverRange: "1.0.x",
			expectErr:   true,
		},
		{
			label:       "With non-matching version list",
			versions:    []string{"1.2.0"},
			semverRange: "1.0.x",
			expectErr:   true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.label, func(t *testing.T) {
			policy, err := NewSemVer(tt.semverRange)
			if err != nil {
				t.Fatalf("returned unexpected error: %s", err)
			}

			latest, err := policy.Latest(tt.versions)
			if tt.expectErr && err == nil {
				t.Fatalf("expecting error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("returned unexpected error: %s", err)
			}

			if latest != tt.expectedVersion {
				t.Errorf("incorrect computed version returned, got '%s', expected '%s'", latest, tt.expectedVersion)
			}
		})
	}
}
