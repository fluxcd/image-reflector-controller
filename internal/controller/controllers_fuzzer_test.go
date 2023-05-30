//go:build gofuzz_libfuzzer
// +build gofuzz_libfuzzer

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

package controller

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/dgraph-io/badger/v3"
	. "github.com/onsi/ginkgo"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	fuzz "github.com/AdaLogics/go-fuzz-headers"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/database"
	"github.com/fluxcd/image-reflector-controller/internal/test"
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

const defaultBinVersion = "1.24"

//go:embed testdata/crd/*.yaml
var testFiles embed.FS

// Fuzz implements a fuzzer that creates pseudo-random objects
// for the ImageRepositoryController to reconcile.
func Fuzz_ImageRepositoryController(f *testing.F) {
	f.Fuzz(func(t *testing.T, seed []byte) {

		initter.Do(initFunc)
		registryServer = test.NewRegistryServer()
		defer registryServer.Close()
		fc := fuzz.NewConsumer(seed)

		imgRepo := test.RegistryName(registryServer)
		repo := imagev1.ImageRepository{}
		err := fc.GenerateStruct(&repo)
		if err != nil {
			return
		}
		repo.Spec.Image = imgRepo

		objectName, err := fc.GetStringFrom("abcdefghijklmnopqrstuvwxyz123456789", 59)
		if err != nil {
			return
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
			return
		}
		err = r.Create(ctx, &repo)
		if err != nil {
			return
		}
		time.Sleep(30 * time.Millisecond)
		err = r.Get(ctx, imageObjectName, &repo)
		if err != nil || repo.Status.LastScanResult != nil {
			return
		}

		polNs, err := fc.GetStringFrom("abcdefghijklmnopqrstuvwxyz123456789", 59)
		if err != nil {
			return
		}
		polName := types.NamespacedName{
			Name:      polNs,
			Namespace: imageObjectName.Namespace,
		}
		pol := imagev1.ImagePolicy{}
		err = fc.GenerateStruct(&pol)
		if err != nil {
			return
		}
		pol.Spec.ImageRepositoryRef.Name = imageObjectName.Name

		pol.Namespace = polName.Namespace
		pol.Name = polName.Name

		ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()

		err = r.Create(ctx, &pol)
		if err != nil {
			return
		}
		time.Sleep(time.Millisecond * 30)
		r.Get(ctx, polName, &pol)
	})
}

// ensureDependencies ensure that:
// a) setup-envtest is installed and a specific version of envtest is deployed.
// b) the embedded crd files are exported onto the "runner container".
//
// The steps above are important as the fuzzers tend to be built in an
// environment (or container) and executed in other.
func ensureDependencies() error {
	// only install dependencies when running inside a container
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		return nil
	}

	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		binVersion := envtestBinVersion()
		cmd := exec.Command("/usr/bin/bash", "-c", fmt.Sprintf(`go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest && \
		/root/go/bin/setup-envtest use -p path %s`, binVersion))

		cmd.Env = append(os.Environ(), "GOPATH=/root/go")
		assetsPath, err := cmd.Output()
		if err != nil {
			return err
		}
		os.Setenv("KUBEBUILDER_ASSETS", string(assetsPath))
	}

	// Output all embedded testdata files to disk.
	embedDirs := []string{"testdata/crd"}
	for _, dir := range embedDirs {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			return fmt.Errorf("mkdir %s: %v", dir, err)
		}

		templates, err := fs.ReadDir(testFiles, dir)
		if err != nil {
			return fmt.Errorf("reading embedded dir: %v", err)
		}

		for _, template := range templates {
			fileName := fmt.Sprintf("%s/%s", dir, template.Name())
			fmt.Println(fileName)

			data, err := testFiles.ReadFile(fileName)
			if err != nil {
				return fmt.Errorf("reading embedded file %s: %v", fileName, err)
			}

			os.WriteFile(fileName, data, 0o644)
			if err != nil {
				return fmt.Errorf("writing %s: %v", fileName, err)
			}
		}
	}

	return nil
}

// initFunc is an init function that is invoked by way of sync.Do.
func initFunc() {
	if err := ensureDependencies(); err != nil {
		panic(fmt.Sprintf("Failed to ensure dependencies: %v", err))
	}

	ctrl.SetLogger(
		zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), zap.Level(zapcore.PanicLevel)),
	)

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("testdata", "crd")},
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
	badgerOpts := badger.DefaultOptions(badgerDir)
	badgerOpts.Logger = nil
	badgerDB, err = badger.Open(badgerOpts)
	if err != nil {
		panic(err)
	}

	imageRepoReconciler = &ImageRepositoryReconciler{
		Client:        k8sMgr.GetClient(),
		Database:      database.NewBadgerDatabase(badgerDB),
		EventRecorder: record.NewFakeRecorder(256),
		patchOptions:  getPatchOptions(imageRepositoryOwnedConditions, "irc"),
	}
	err = imageRepoReconciler.SetupWithManager(k8sMgr, ImageRepositoryReconcilerOptions{})
	if err != nil {
		panic(err)
	}

	imagePolicyReconciler = &ImagePolicyReconciler{
		Client:        k8sMgr.GetClient(),
		Database:      database.NewBadgerDatabase(badgerDB),
		EventRecorder: record.NewFakeRecorder(256),
		patchOptions:  getPatchOptions(imagePolicyOwnedConditions, "irc"),
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

func envtestBinVersion() string {
	if binVersion := os.Getenv("ENVTEST_KUBERNETES_VERSION"); binVersion != "" {
		return binVersion
	}
	return defaultBinVersion
}
