//go:build gofuzz
// +build gofuzz

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

package controllers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/dgraph-io/badger/v3"
	. "github.com/onsi/ginkgo"

	//. "github.com/onsi/gomega"
	"github.com/google/go-containerregistry/pkg/registry"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	"github.com/fluxcd/image-reflector-controller/internal/database"
)

const (
	timeout                = time.Second * 30
	contextTimeout         = time.Second * 10
	interval               = time.Second * 1
	reconciliationInterval = time.Second * 2
)

var registryServer *httptest.Server
var cfg *rest.Config
var k8sClient client.Client
var k8sMgr ctrl.Manager
var stopManager func()
var imageRepoReconciler *ImageRepositoryReconciler
var imagePolicyReconciler *ImagePolicyReconciler
var testEnv *envtest.Environment
var badgerDir string
var badgerDB *badger.DB
var initter sync.Once

// createKUBEBUILDER_ASSETS runs "setup-envtest use"
// and returns the path of the 3 binaries
func createKUBEBUILDER_ASSETS() string {
	out, err := exec.Command("setup-envtest", "use").Output()
	if err != nil {
		panic(err)
	}

	// split the output to get the path:
	splitString := strings.Split(string(out), " ")
	binPath := strings.TrimSuffix(splitString[len(splitString)-1], "\n")
	if err != nil {
		panic(err)
	}
	return binPath
}

// initFunc is an init function that is invoked by
// way of sync.Do.
func initFunc() {
	kubebuilder_assets := createKUBEBUILDER_ASSETS()
	os.Setenv("KUBEBUILDER_ASSETS", kubebuilder_assets)

	ctrl.SetLogger(
		zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), zap.Level(zapcore.PanicLevel)),
	)

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		panic(err)
	}
	if cfg == nil {
		panic("cfg is nill")
	}

	err = imagev1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}

	k8sMgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		panic(err)
	}

	badgerDir, err = ioutil.TempDir(os.TempDir(), "badger")
	if err != nil {
		panic(err)
	}
	badgerDB, err = badger.Open(badger.DefaultOptions(badgerDir))
	if err != nil {
		panic(err)
	}

	imageRepoReconciler = &ImageRepositoryReconciler{
		Client:   k8sMgr.GetClient(),
		Scheme:   scheme.Scheme,
		Database: database.NewBadgerDatabase(badgerDB),
	}
	err = imageRepoReconciler.SetupWithManager(k8sMgr, ImageRepositoryReconcilerOptions{})
	if err != nil {
		panic(err)
	}

	imagePolicyReconciler = &ImagePolicyReconciler{
		Client:   k8sMgr.GetClient(),
		Scheme:   scheme.Scheme,
		Database: database.NewBadgerDatabase(badgerDB),
	}
	err = imagePolicyReconciler.SetupWithManager(k8sMgr, ImagePolicyReconcilerOptions{})
	if err != nil {
		panic(err)
	}

	mgrContext, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	go func() {
		err = k8sMgr.Start(mgrContext)
		if err != nil {
			panic(err)
		}
	}()
	stopManager = cancel

	k8sClient = k8sMgr.GetClient()
	if k8sClient == nil {
		panic("k8sClient is nil")
	}
}

func registryName(srv *httptest.Server) string {
	if strings.HasPrefix(srv.URL, "https://") {
		return strings.TrimPrefix(srv.URL, "https://")
	} // else assume HTTP
	return strings.TrimPrefix(srv.URL, "http://")
}

// Fuzz implements a fuzzer that creates pseudo-random objects.
func Fuzz(data []byte) int {
	initter.Do(initFunc)
	registryServer = newRegistryServer()
	defer registryServer.Close()
	f := fuzz.NewConsumer(data)

	imgRepo := registryName(registryServer)
	repo := imagev1.ImageRepository{}
	err := f.GenerateStruct(&repo)
	if err != nil {
		return 0
	}
	repo.Spec.Image = imgRepo

	objectName, err := f.GetStringFrom("abcdefghijklmnopqrstuvwxyz123456789", 59)
	if err != nil {
		return 0
	}
	imageObjectName := types.NamespacedName{
		Name:      objectName,
		Namespace: "default",
	}
	repo.Name = imageObjectName.Name
	repo.Namespace = imageObjectName.Namespace

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	r := imageRepoReconciler
	if r == nil {
		panic("r is nil")
	}
	err = r.Create(ctx, &repo)
	if err != nil {
		return 0
	}
	time.Sleep(30 * time.Millisecond)
	err = r.Get(ctx, imageObjectName, &repo)
	if err != nil || repo.Status.LastScanResult != nil {
		panic("Failed1")
	}

	polNs, err := f.GetStringFrom("abcdefghijklmnopqrstuvwxyz123456789", 59)
	if err != nil {
		return 0
	}
	polName := types.NamespacedName{
		Name:      polNs,
		Namespace: imageObjectName.Namespace,
	}
	pol := imagev1.ImagePolicy{}
	err = f.GenerateStruct(&pol)
	if err != nil {
		return 0
	}
	pol.Spec.ImageRepositoryRef.Name = imageObjectName.Name

	pol.Namespace = polName.Namespace
	pol.Name = polName.Name

	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	err = r.Create(ctx, &pol)
	if err != nil {
		return 0
	}
	time.Sleep(time.Millisecond * 30)
	err = r.Get(ctx, polName, &pol)
	if err != nil {
		panic(err)
	}
	return 1
}

// Taken from here: https://github.com/fluxcd/image-reflector-controller/blob/main/controllers/registry_test.go#L62
func newRegistryServer() *httptest.Server {
	regHandler := registry.New()
	srv := httptest.NewServer(&tagListHandler{
		registryHandler: regHandler,
		imagetags:       convenientTags,
	})
	return srv
}

// tje tagListHandler is taken from here:
// https://github.com/fluxcd/image-reflector-controller/blob/main/controllers/registry_test.go#L62

type tagListHandler struct {
	registryHandler http.Handler
	imagetags       map[string][]string
}

type tagListResult struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// Take from here: https://github.com/fluxcd/image-reflector-controller/blob/main/controllers/registry_test.go#L126
// and modified to not include any of the BDD APIs
func (h *tagListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if withoutTagsList := strings.TrimSuffix(r.URL.Path, "/tags/list"); r.Method == "GET" && withoutTagsList != r.URL.Path {
		repo := strings.TrimPrefix(withoutTagsList, "/v2/")
		if tags, ok := h.imagetags[repo]; ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			result := tagListResult{
				Name: repo,
				Tags: tags,
			}
			err := json.NewEncoder(w).Encode(result)
			if err != nil {
				panic(err)
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

var convenientTags = map[string][]string{
	"convenient": []string{
		"tag1", "tag2",
	},
}
