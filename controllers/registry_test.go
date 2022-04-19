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

package controllers

import (
	"net/http/httptest"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/image-reflector-controller/internal/test"
)

var _ = Context("Registry handler", func() {

	It("serves a tag list", func() {
		srv := test.NewRegistryServer()
		defer srv.Close()

		uploadedTags := []string{"tag1", "tag2"}
		repoString, err := test.LoadImages(srv, "testimage", uploadedTags)
		Expect(err).ToNot(HaveOccurred())
		repo, _ := name.NewRepository(repoString)

		tags, err := remote.List(repo)
		Expect(err).ToNot(HaveOccurred())
		Expect(tags).To(Equal(uploadedTags))
	})
})

var _ = Context("Authentication handler", func() {

	var registryServer *httptest.Server
	var username, password string

	BeforeEach(func() {
		username = "user"
		password = "password1"
		registryServer = test.NewAuthenticatedRegistryServer(username, password)
	})

	AfterEach(func() {
		registryServer.Close()
	})

	It("rejects requests without authentication", func() {
		repo, err := name.NewRepository(test.RegistryName(registryServer) + "/convenient")
		Expect(err).ToNot(HaveOccurred())
		_, err = remote.List(repo)
		Expect(err).To(HaveOccurred())
	})

	It("accepts requests with correct authentication", func() {
		repo, err := name.NewRepository(test.RegistryName(registryServer) + "/convenient")
		Expect(err).ToNot(HaveOccurred())
		auth := &authn.Basic{
			Username: username,
			Password: password,
		}
		_, err = remote.List(repo, remote.WithAuth(auth))
		Expect(err).ToNot(HaveOccurred())
	})
})
