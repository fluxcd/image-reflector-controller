/*
Copyright 2022 The Flux authors

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
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	tftestenv "github.com/fluxcd/image-reflector-controller/tests/tftestenv"
	// flag "github.com/spf13/pflag"
)

const (
	// kubeconfigPath is the path where the cluster kubeconfig is written to and
	// used from.
	kubeconfigPath = "./build/kubeconfig"
	// fluxInstallManifestPath is the flux installation manifest file path. It
	// is generated before running the Go test.
	fluxInstallManifestPath = "./build/flux.yaml"

	resultWaitTimeout = 20 * time.Second
	operationTimeout  = 10 * time.Second
)

var (
	// retain flag to prevent destroy and retaining the created infrastructure.
	retain = flag.Bool("retain", true, "retain the infrastructure for debugging purposes")

	// existing flag to use existing infrastructure terraform state.
	existing = flag.Bool("existing", true, "use existing infrastructure state for debugging purposes")

	// verbose
	verbose = flag.Bool("verbose", true, "verbose output of the environment setup")

	// the particular provider that test will run
	provider = flag.String("provider", "", "verbose output of the environment setup")

	// testRepoURL is the URL of the test repository.
	testRepoURL string

	// testEnv is the test environment. It contains test infrastructure and
	// kubernetes client of the created cluster.
	testEnv *tftestenv.Environment
)

// ProviderInfo contains the specific details needed to run the test for each provider
type ProviderInfo struct {
	terraformPath        string
	loginCmd             string
	createKubeconfigFunc tftestenv.CreateKubeconfig
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func TestMain(m *testing.M) {
	flag.Parse()
	ctx := context.TODO()

	if *provider == "" {
		log.Fatalf("Please specify a provider")
	}

	providerInfo, err := getProviderInfo(ctx, *provider)
	if err != nil {
		panic(err)
	}

	// Construct scheme to be added to the kubeclient.
	scheme := runtime.NewScheme()
	err = imagev1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	// Create environment.
	envOpts := []tftestenv.EnvironmentOption{
		tftestenv.WithVerbose(*verbose),
		tftestenv.WithRetain(*retain),
		tftestenv.WithExisting(*existing),
		tftestenv.WithCreateKubeconfig(providerInfo.createKubeconfigFunc),
	}
	testEnv, err = tftestenv.New(ctx, scheme, providerInfo.terraformPath, kubeconfigPath, envOpts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to provision the test infrastructure: %v", err))
	}

	// Extract values from terraform state output.
	output, err := testEnv.StateOutput(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to get the terraform state output: %v", err))
	}

	testRepoURL = output["repository_url"].Value.(string)
	testRepoURL = testRepoURL + "/random"

	// Registry login.
	if err := tftestenv.RegistryLogin(ctx, testRepoURL, providerInfo.loginCmd); err != nil {
		panic(fmt.Sprintf("Failed to log into the registry: %v", err))
	}

	// Create and push test images.
	if err := tftestenv.CreateAndPushImages(testRepoURL, []string{"v0.1.0", "v0.1.2", "v0.1.3", "v0.1.4"}); err != nil {
		panic(fmt.Sprintf("Failed to create and push images: %v", err))
	}

	log.Println("Installing flux")
	tftestenv.InstallFlux(ctx, kubeconfigPath, fluxInstallManifestPath)

	code := m.Run()

	// log.Println("Uninstalling flux")
	// utils.UninstallFlux(ctx, kubeconfigPath, fluxInstallManifestPath)

	testEnv.Stop(ctx)
	os.Exit(code)
}

func getProviderInfo(ctx context.Context, provider string) (*ProviderInfo, error) {
	var terraformPath string
	var createKubeConfig tftestenv.CreateKubeconfig
	var loginCmd string

	switch provider {
	case "aws":
		terraformPath = "terraform/eks"
		createKubeConfig = tftestenv.CreateEKSKubeconfig
		loginCmd = "aws ecr get-login-password --region us-east-2 | docker login --username AWS --password-stdin %s"
	case "azure":
		terraformPath = "terraform/aks"
		createKubeConfig = tftestenv.CreateAKSKubeConfig
		loginCmd = "az acr login --name %s"
	default:
		return nil, errors.New(fmt.Sprintf("unable to get info for unknown provider '%s'", provider))
	}

	return &ProviderInfo{
		terraformPath:        terraformPath,
		createKubeconfigFunc: createKubeConfig,
		loginCmd:             loginCmd,
	}, nil
}
