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

package test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/gomega"
)

func TestRegistryHandler(t *testing.T) {
	g := NewWithT(t)

	srv := NewRegistryServer()
	defer srv.Close()

	uploadedTags := []string{"tag1", "tag2"}
	repoString, _, err := LoadImages(srv, "testimage", uploadedTags)
	g.Expect(err).ToNot(HaveOccurred())
	repo, _ := name.NewRepository(repoString)

	tags, err := remote.List(repo)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(tags).To(Equal(uploadedTags))
}

func TestAuthenticationHandler(t *testing.T) {
	username, password := "user", "password1"

	tests := []struct {
		name     string
		authInfo *authn.Basic
		wantErr  bool
	}{
		{
			name:     "without auth info",
			authInfo: nil,
			wantErr:  true,
		},
		{
			name: "with auth info",
			authInfo: &authn.Basic{
				Username: username,
				Password: password,
			},
			wantErr: false,
		},
	}

	registryServer := NewAuthenticatedRegistryServer(username, password)
	defer registryServer.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			repo, err := name.NewRepository(RegistryName(registryServer) + "/convenient")
			g.Expect(err).ToNot(HaveOccurred())

			var listErr error
			if tt.authInfo != nil {
				_, listErr = remote.List(repo, remote.WithAuth(tt.authInfo))
			} else {
				_, listErr = remote.List(repo)
			}

			if tt.wantErr {
				g.Expect(listErr).To(HaveOccurred())
			} else {
				g.Expect(listErr).ToNot(HaveOccurred())
			}
		})
	}
}
