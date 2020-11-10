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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("Registry handler", func() {

	It("serves a tag list", func() {
		srv := newRegistryServer()
		defer srv.Close()

		uploadedTags := []string{"tag1", "tag2"}
		repoString := loadImages(srv, "testimage", uploadedTags)
		repo, _ := name.NewRepository(repoString)

		tags, err := remote.List(repo)
		Expect(err).ToNot(HaveOccurred())
		Expect(tags).To(Equal(uploadedTags))
	})
})

// ---

// set up a local registry for testing scanning
func newRegistryServer() *httptest.Server {
	regHandler := registry.New()
	srv := httptest.NewServer(&tagListHandler{
		registryHandler: regHandler,
		imagetags:       map[string][]string{},
	})
	return srv
}

// loadImages uploads images to the local registry, and returns the
// image repo name.
func loadImages(srv *httptest.Server, imageName string, versions []string) string {
	registry := strings.TrimPrefix(srv.URL, "http://")
	imgRepo := registry + "/" + imageName
	for _, tag := range versions {
		imgRef, err := name.NewTag(imgRepo + ":" + tag)
		Expect(err).ToNot(HaveOccurred())
		img, err := random.Image(512, 1)
		Expect(err).ToNot(HaveOccurred())
		Expect(remote.Write(imgRef, img)).To(Succeed())
	}
	return imgRepo
}

// the go-containerregistry test registry implementation does not
// serve /myimage/tags/list. Until it does, I'm adding this handler.
// NB:
// - assumes repo name is a single element
// - assumes no overwriting tags

type tagListHandler struct {
	registryHandler http.Handler
	imagetags       map[string][]string
}

type tagListResult struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (h *tagListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// a tag list request has a path like: /v2/<repo>/tags/list
	if withoutTagsList := strings.TrimSuffix(r.URL.Path, "/tags/list"); r.Method == "GET" && withoutTagsList != r.URL.Path {
		repo := strings.TrimPrefix(withoutTagsList, "/v2/")
		if tags, ok := h.imagetags[repo]; ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			result := tagListResult{
				Name: repo,
				Tags: tags,
			}
			Expect(json.NewEncoder(w).Encode(result)).To(Succeed())
			println("Requested tags", repo, strings.Join(tags, ", "))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// record the fact of a PUT to a tag; the path looks like: /v2/<repo>/manifests/<tag>
	h.registryHandler.ServeHTTP(w, r)
	if r.Method == "PUT" {
		pathElements := strings.Split(r.URL.Path, "/")
		if len(pathElements) == 5 && pathElements[1] == "v2" && pathElements[3] == "manifests" {
			repo, tag := pathElements[2], pathElements[4]
			println("Recording tag", repo, tag)
			h.imagetags[repo] = append(h.imagetags[repo], tag)
		}
	}
}
