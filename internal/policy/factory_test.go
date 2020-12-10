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

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
)

func TestFactory_PolicerFromSpec(t *testing.T) {
	// With invalid ImagePolicyChoice
	_, err := PolicerFromSpec(imagev1.ImagePolicyChoice{})
	if err == nil {
		t.Error("expected error, got nil")
	}

	// With SemVerPolicy
	_, err = PolicerFromSpec(imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "1.0.x"}})
	if err != nil {
		t.Error("should not return error")
	}

	// With AlphabeticalPolicy
	_, err = PolicerFromSpec(imagev1.ImagePolicyChoice{Alphabetical: &imagev1.AlphabeticalPolicy{}})
	if err != nil {
		t.Error("should not return error")
	}
}
