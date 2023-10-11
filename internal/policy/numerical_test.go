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
	"math/rand"
	"testing"
)

func TestNewNumerical(t *testing.T) {
	cases := []struct {
		label     string
		order     string
		expectErr bool
	}{
		{
			label: "With valid empty order",
			order: "",
		},
		{
			label: "With valid asc order",
			order: NumericalOrderAsc,
		},
		{
			label: "With valid desc order",
			order: NumericalOrderDesc,
		},
		{
			label:     "With invalid order",
			order:     "invalid",
			expectErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.label, func(t *testing.T) {
			_, err := NewNumerical(tt.order)
			if tt.expectErr && err == nil {
				t.Fatalf("expecting error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("returned unexpected error: %s", err)
			}
		})
	}
}

func TestNumerical_Latest(t *testing.T) {
	cases := []struct {
		label           string
		order           string
		versions        []string
		expectedVersion string
		expectErr       bool
	}{
		{
			label:           "With unordered list of integers ascending",
			versions:        shuffle([]string{"-62", "-88", "73", "72", "15", "16", "15", "29", "-33", "-91"}),
			expectedVersion: "73",
		},
		{
			label:           "With unordered list of integers descending",
			versions:        shuffle([]string{"5", "-8", "-78", "25", "70", "-4", "80", "92", "-20", "-24"}),
			order:           NumericalOrderDesc,
			expectedVersion: "-78",
		},
		{
			label:           "With unordered list of floats ascending",
			versions:        shuffle([]string{"47.40896403322944", "-27.8520927455902", "-27.930666514224427", "-31.352485948094568", "-50.41072694704882", "-21.962849842263736", "24.71884721436865", "-39.99177354004344", "53.47333823144817", "3.2008658570411086"}),
			expectedVersion: "53.47333823144817",
		},
		{
			label:           "With unordered list of floats descending",
			versions:        shuffle([]string{"-65.27202780220686", "57.82948329142309", "22.40184684363291", "-86.36934305697784", "-90.29082099756083", "-12.041712603564264", "77.70488240399305", "-38.98425003883552", "16.06867070412028", "53.735674335181216"}),
			order:           NumericalOrderDesc,
			expectedVersion: "-90.29082099756083",
		},
		{
			label:           "With Unix Timestamps ascending",
			versions:        shuffle([]string{"1606234201", "1606364286", "1606334092", "1606334284", "1606334201"}),
			expectedVersion: "1606364286",
		},
		{
			label:           "With Unix Timestamps descending",
			versions:        shuffle([]string{"1606234201", "1606364286", "1606334092", "1606334284", "1606334201"}),
			order:           NumericalOrderDesc,
			expectedVersion: "1606234201",
		},
		{
			label:           "With single value ascending",
			versions:        []string{"1"},
			expectedVersion: "1",
		},
		{
			label:           "With single value descending",
			versions:        []string{"1"},
			order:           NumericalOrderDesc,
			expectedVersion: "1",
		},
		{
			label:     "With invalid numerical value",
			versions:  []string{"0", "1a", "b"},
			expectErr: true,
		},
		{
			label:     "Empty version list",
			versions:  []string{},
			expectErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.label, func(t *testing.T) {
			policy, err := NewNumerical(tt.order)
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

func shuffle(list []string) []string {
	rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	return list
}
