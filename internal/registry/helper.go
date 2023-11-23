/*
Copyright 2023 The Flux authors

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

package registry

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

// ParseImageReference parses the given reference string into a container
// registry repository reference.
func ParseImageReference(refs string) (name.Reference, error) {
	if s := strings.Split(refs, "://"); len(s) > 1 {
		return nil, fmt.Errorf("image reference value should not include URL scheme; remove '%s://'", s[0])
	}

	ref, err := name.ParseReference(refs)
	if err != nil {
		return nil, err
	}

	imageName := strings.TrimPrefix(refs, ref.Context().RegistryStr())
	if s := strings.Split(imageName, ":"); len(s) > 1 {
		return nil, fmt.Errorf(".spec.image value should not contain a tag; remove ':%s'", s[1])
	}

	return ref, nil
}
