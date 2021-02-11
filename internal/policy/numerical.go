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
	"strconv"
)

const (
	// NumericalOrderAsc ascending order
	NumericalOrderAsc = "ASC"
	// NumericalOrderDesc descending order
	NumericalOrderDesc = "DESC"
)

// Numerical representes a Numerical ordering policy
type Numerical struct {
	Order string
}

// NewNumerical constructs a Numerical object validating the provided
// order argument
func NewNumerical(order string) (*Numerical, error) {
	switch order {
	case "":
		order = NumericalOrderAsc
	case NumericalOrderAsc, NumericalOrderDesc:
		break
	default:
		return nil, fmt.Errorf("invalid order argument provided: '%s', must be one of: %s, %s", order, NumericalOrderAsc, NumericalOrderDesc)
	}

	return &Numerical{
		Order: order,
	}, nil
}

// Latest returns latest version from a provided list of strings
func (p *Numerical) Latest(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("version list argument cannot be empty")
	}

	var latest string
	var pv float64
	for i, version := range versions {
		cv, err := strconv.ParseFloat(version, 64)
		if err != nil {
			return "", fmt.Errorf("failed to parse invalid numeric value '%s'", version)
		}

		switch {
		case i == 0:
			break // First iteration, nothing to compare
		case p.Order == NumericalOrderAsc && cv < pv, p.Order == NumericalOrderDesc && cv > pv:
			continue
		}

		latest = version
		pv = cv
	}

	return latest, nil
}
