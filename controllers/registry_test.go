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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/gomega"
)

func TestRegistryHandler(t *testing.T) {
	g := NewWithT(t)

	srv := newRegistryServer()
	defer srv.Close()

	uploadedTags := []string{"tag1", "tag2"}
	repoString, err := loadImages(srv, "testimage", uploadedTags)
	g.Expect(err).ToNot(HaveOccurred())
	repo, _ := name.NewRepository(repoString)

	tags, err := remote.List(repo)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(tags).To(Equal(uploadedTags))
}

// ---

// pre-populated db of tags, so it's not necessary to upload images to
// get results from remote.List.
var convenientTags = map[string][]string{
	"convenient": []string{
		"tag1", "tag2",
	},
}

// set up a local registry for testing scanning
func newRegistryServer() *httptest.Server {
	regHandler := registry.New()
	srv := httptest.NewServer(&tagListHandler{
		registryHandler: regHandler,
		imagetags:       convenientTags,
	})
	return srv
}

func newAuthenticatedRegistryServer(username, pass string) *httptest.Server {
	regHandler := registry.New()
	regHandler = &tagListHandler{
		registryHandler: regHandler,
		imagetags:       convenientTags,
	}
	regHandler = &authHandler{
		registryHandler: regHandler,
		allowedUser:     username,
		allowedPass:     pass,
	}
	srv := httptest.NewServer(regHandler)
	return srv
}

// Get the registry part of an image from the registry server
func registryName(srv *httptest.Server) string {
	if strings.HasPrefix(srv.URL, "https://") {
		return strings.TrimPrefix(srv.URL, "https://")
	} // else assume HTTP
	return strings.TrimPrefix(srv.URL, "http://")
}

// loadImages uploads images to the local registry, and returns the
// image repo
// name. https://github.com/google/go-containerregistry/blob/v0.1.1/pkg/registry/compatibility_test.go
// has an example of loading a test registry with a random image.
func loadImages(srv *httptest.Server, imageName string, versions []string, options ...remote.Option) (string, error) {
	imgRepo := registryName(srv) + "/" + imageName
	for _, tag := range versions {
		imgRef, err := name.NewTag(imgRepo + ":" + tag)
		if err != nil {
			return imgRepo, err
		}
		img, err := random.Image(512, 1)
		if err != nil {
			return imgRepo, err
		}
		if err := remote.Write(imgRef, img, options...); err != nil {
			return imgRepo, err
		}
	}
	return imgRepo, nil
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
			if err := json.NewEncoder(w).Encode(result); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
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

// there's no authentication in go-containerregistry/pkg/registry;
// this wrapper adds basic auth to a registry handler. NB: the
// important thing is to be able to test that the credentials get from
// the secret to the registry API library; it's assumed that the
// registry API library does e.g., OAuth2 correctly.  See
// https://tools.ietf.org/html/rfc7617 regarding basic authentication.

type authHandler struct {
	allowedUser, allowedPass string
	registryHandler          http.Handler
}

// ServeHTTP serves a request which needs authentication.
func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Add("WWW-Authenticate", `Basic realm="Registry"`)
		w.WriteHeader(401)
		return
	}
	if !strings.HasPrefix(authHeader, "Basic ") {
		w.WriteHeader(403)
		w.Write([]byte(`Authorization header does not being with "Basic "`))
		return
	}
	namePass, err := base64.StdEncoding.DecodeString(authHeader[6:])
	if err != nil {
		w.WriteHeader(403)
		w.Write([]byte(`Authorization header doesn't appear to be base64-encoded`))
		return
	}
	namePassSlice := strings.SplitN(string(namePass), ":", 2)
	if len(namePassSlice) != 2 {
		w.WriteHeader(403)
		w.Write([]byte(`Authorization header doesn't appear to be colon-separated value `))
		w.Write(namePass)
		return
	}
	if namePassSlice[0] != h.allowedUser || namePassSlice[1] != h.allowedPass {
		w.WriteHeader(403)
		w.Write([]byte(`Authorization failed: wrong username or password`))
		return
	}
	h.registryHandler.ServeHTTP(w, r)
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

	registryServer := newAuthenticatedRegistryServer(username, password)
	defer registryServer.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			repo, err := name.NewRepository(registryName(registryServer) + "/convenient")
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
