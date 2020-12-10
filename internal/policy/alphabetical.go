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
	"fmt"
	"sort"
)

const (
	// AlphabeticalOrderAsc ascending order
	AlphabeticalOrderAsc = "ASC"
	// AlphabeticalOrderDesc descending order
	AlphabeticalOrderDesc = "DESC"
)

// Alphabetical representes a alphabetical ordering policy
type Alphabetical struct {
	Order string
}

// NewAlphabetical constructs a Alphabetical object validating the provided semver constraint
func NewAlphabetical(order string) (*Alphabetical, error) {
	switch order {
	case "":
		order = AlphabeticalOrderAsc
	case AlphabeticalOrderAsc, AlphabeticalOrderDesc:
		break
	default:
		return nil, fmt.Errorf("invalid order argument provided: '%s', must be one of: %s, %s", order, AlphabeticalOrderAsc, AlphabeticalOrderDesc)
	}

	return &Alphabetical{
		Order: order,
	}, nil
}

// Latest returns latest version from a provided list of strings
func (p *Alphabetical) Latest(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("version list argument cannot be empty")
	}

	sorted := sort.StringSlice(versions)
	if p.Order == AlphabeticalOrderDesc {
		sort.Sort(sorted)
	} else {
		sort.Sort(sort.Reverse(sorted))
	}

	return sorted[0], nil
}
