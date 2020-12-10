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

	"github.com/Masterminds/semver/v3"
	"github.com/fluxcd/pkg/version"
)

// SemVer representes a SemVer policy
type SemVer struct {
	Range string

	constraint *semver.Constraints
}

// NewSemVer constructs a SemVer object validating the provided semver constraint
func NewSemVer(r string) (*SemVer, error) {
	constraint, err := semver.NewConstraint(r)
	if err != nil {
		return nil, err
	}

	return &SemVer{
		Range:      r,
		constraint: constraint,
	}, nil
}

// Latest returns latest version from a provided list of strings
func (p *SemVer) Latest(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", fmt.Errorf("version list argument cannot be empty")
	}

	var latestVersion *semver.Version
	for _, ver := range versions {
		if v, err := version.ParseVersion(ver); err == nil {
			if p.constraint.Check(v) && (latestVersion == nil || v.GreaterThan(latestVersion)) {
				latestVersion = v
			}
		}
	}
	if latestVersion != nil {
		return latestVersion.Original(), nil
	}
	return "", fmt.Errorf("unable to determine latest version from provided list")
}
