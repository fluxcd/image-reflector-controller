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
	"strings"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
)

// PolicerFromSpec constructs a new policy object based on
func PolicerFromSpec(choice imagev1.ImagePolicyChoice) (Policer, error) {
	var p Policer
	var err error
	switch {
	case choice.SemVer != nil:
		p, err = NewSemVer(choice.SemVer.Range)
	case choice.Alphabetical != nil:
		p, err = NewAlphabetical(strings.ToUpper(choice.Alphabetical.Order))
	default:
		return nil, fmt.Errorf("given ImagePolicyChoice object is invalid")
	}

	return p, err
}
