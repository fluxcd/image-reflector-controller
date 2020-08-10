/*
Copyright 2020 The Flux CD contributors.

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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	imagev1alpha1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// for Eventually
const (
	timeout  = time.Second * 30
	interval = time.Second * 1
	// indexInterval = time.Second * 1
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sMgr ctrl.Manager
var imageRepoReconciler *ImageRepositoryReconciler
var imagePolicyReconciler *ImagePolicyReconciler
var testEnv *envtest.Environment
var registryServer *httptest.Server

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = imagev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = imagev1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sMgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	db := NewDatabase()

	imageRepoReconciler = &ImageRepositoryReconciler{
		Client:   k8sMgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("ImageRepository"),
		Scheme:   scheme.Scheme,
		Database: db,
	}
	Expect(imageRepoReconciler.SetupWithManager(k8sMgr)).To(Succeed())

	imagePolicyReconciler = &ImagePolicyReconciler{
		Client:   k8sMgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("ImagePolicy"),
		Scheme:   scheme.Scheme,
		Database: db,
	}
	Expect(imagePolicyReconciler.SetupWithManager(k8sMgr)).To(Succeed())

	// this must be started for the caches to be running, and thereby
	// for the client to be usable.
	go func() {
		err = k8sMgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sMgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	// set up a local registry for testing scanning
	regHandler := registry.New()
	registryServer = httptest.NewServer(&tagListHandler{
		registryHandler: regHandler,
		imagetags:       map[string][]string{},
	})

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
	registryServer.Close()
})

// ---

// the go-containerregistry test regsitry implementation does not
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
