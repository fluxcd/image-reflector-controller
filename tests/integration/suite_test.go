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

package integration

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
	"k8s.io/apimachinery/pkg/runtime"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/test-infra/tftestenv"
)

const (
	// terraformPathAWS is the path to the terraform working directory
	// containing the aws terraform configurations.
	terraformPathAWS = "./terraform/aws"
	// terraformPathAzure is the path to the terraform working directory
	// containing the azure terraform configurations.
	terraformPathAzure = "./terraform/azure"
	// terraformPathGCP is the path to the terraform working directory
	// containing the gcp terraform configurations.
	terraformPathGCP = "./terraform/gcp"
	// kubeconfigPath is the path where the cluster kubeconfig is written to and
	// used from.
	kubeconfigPath = "./build/kubeconfig"
	// fluxInstallManifestPath is the flux installation manifest file path. It
	// is generated before running the Go test.
	fluxInstallManifestPath = "./build/flux.yaml"

	resultWaitTimeout = 30 * time.Second
)

var (
	// supportedProviders are the providers supported by the test.
	supportedProviders = []string{"aws", "azure", "gcp"}

	// targetProvider is the name of the kubernetes provider to test against.
	targetProvider = flag.String("provider", "", fmt.Sprintf("name of the provider %v", supportedProviders))

	// retain flag to prevent destroy and retaining the created infrastructure.
	retain = flag.Bool("retain", false, "retain the infrastructure for debugging purposes")

	// existing flag to use existing infrastructure terraform state.
	existing = flag.Bool("existing", false, "use existing infrastructure state for debugging purposes")

	// verbose flag to enable output of terraform execution.
	verbose = flag.Bool("verbose", false, "verbose output of the environment setup")

	// destroyOnly flag to destroy any provisioned infrastructure.
	destroyOnly = flag.Bool("destroy-only", false, "run in destroy-only mode and delete any existing infrastructure")

	// testRepos is a map of registry common name and URL of the test
	// repositories. This is used as the test cases to run the tests against.
	// The registry common name need not be the actual registry address but an
	// identifier to identify the test case without logging any sensitive
	// account IDs in the subtest names.
	// For example, map[string]string{"ecr", "xxxxx.dkr.ecr.xxxx.amazonaws.com/foo:v1"}
	// would result in subtest name TestImageRepositoryScanAWS/ecr.
	testRepos map[string]string

	// testEnv is the test environment. It contains test infrastructure and
	// kubernetes client of the created cluster.
	testEnv *tftestenv.Environment

	// testImageTags are the tags used in the test for the generated images.
	testImageTags = []string{"v0.1.0", "v0.1.2", "v0.1.3", "v0.1.4"}
)

// registryLoginFunc is used to perform registry login against a provider based
// on the terraform state output values. It returns a map of registry common
// name and test repositories to test against, read from the terraform state
// output.
type registryLoginFunc func(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error)

// pushTestImages is used to push local flux test images to a remote registry
// after logging in using registryLoginFunc. It takes a map of image name and
// local images and terraform state output. The local images are retagged and
// pushed to a corresponding registry repository for the image.
type pushTestImages func(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error)

// ProviderConfig is the test configuration of a supported cloud provider to run
// the tests against.
type ProviderConfig struct {
	// terraformPath is the path to the directory containing the terraform
	// configurations of the provider.
	terraformPath string
	// registryLogin is used to perform registry login.
	registryLogin registryLoginFunc
	// createKubeconfig is used to create kubeconfig of a cluster.
	createKubeconfig tftestenv.CreateKubeconfig
	// pushFluxTestImages is used to push flux test images to a remote registry.
	pushFluxTestImages pushTestImages
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

	// Validate the provider.
	if *targetProvider == "" {
		log.Fatalf("-provider flag must be set to one of %v", supportedProviders)
	}
	var supported bool
	for _, p := range supportedProviders {
		if p == *targetProvider {
			supported = true
		}
	}
	if !supported {
		log.Fatalf("Unsupported provider %q, must be one of %v", *targetProvider, supportedProviders)
	}

	providerCfg := getProviderConfig(*targetProvider)
	if providerCfg == nil {
		log.Fatalf("Failed to get provider config for %q", *targetProvider)
	}

	// Run destroy-only mode if enabled.
	if *destroyOnly {
		log.Println("Running in destroy-only mode...")
		envOpts := []tftestenv.EnvironmentOption{
			tftestenv.WithVerbose(*verbose),
			// Ignore any state lock in destroy-only mode.
			tftestenv.WithTfDestroyOptions(tfexec.Lock(false)),
		}
		if err := tftestenv.Destroy(ctx, providerCfg.terraformPath, envOpts...); err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	// Check the test app image.
	ircImg := os.Getenv("TEST_IMG")
	if ircImg == "" {
		log.Fatal("TEST_IMG must be set to the test image-reflector-controller image, cannot be empty")
	}

	localImgs := map[string]string{
		"image-reflector-controller": ircImg,
	}

	// Construct scheme to be added to the kubeclient.
	scheme := runtime.NewScheme()
	err := imagev1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	// Initialize with non-zero exit code to indicate failure by default unless
	// set by a successful test run.
	exitCode := 1

	// Create environment.
	envOpts := []tftestenv.EnvironmentOption{
		tftestenv.WithVerbose(*verbose),
		tftestenv.WithRetain(*retain),
		tftestenv.WithExisting(*existing),
		tftestenv.WithCreateKubeconfig(providerCfg.createKubeconfig),
	}
	testEnv, err = tftestenv.New(ctx, scheme, providerCfg.terraformPath, kubeconfigPath, envOpts...)
	if err != nil {
		panic(fmt.Sprintf("Failed to provision the test infrastructure: %v", err))
	}

	// Stop the environment before exit.
	defer func() {
		if err := testEnv.Stop(ctx); err != nil {
			log.Printf("Failed to stop environment: %v", err)
			exitCode = 1
		}

		// Log the panic error before exit to surface the cause of panic.
		if err := recover(); err != nil {
			log.Printf("panic: %v", err)
		}
		os.Exit(exitCode)
	}()

	// Get terraform state output.
	output, err := testEnv.StateOutput(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to get the terraform state output: %v", err))
	}

	testRepos, err = providerCfg.registryLogin(ctx, output)
	if err != nil {
		panic(fmt.Sprintf("Failed to log into registry: %v", err))
	}

	pushedImages, err := providerCfg.pushFluxTestImages(ctx, localImgs, output)
	if err != nil {
		panic(fmt.Sprintf("Failed to push test images: %v", err))
	}

	// Create and push test images.
	if err := tftestenv.CreateAndPushImages(testRepos, testImageTags); err != nil {
		panic(fmt.Sprintf("Failed to create and push images: %v", err))
	}

	// Update flux install manifests with the pushed test images.
	if err := updateAndBuildFluxInstallManifests(ctx, pushedImages); err != nil {
		panic(fmt.Sprintf("Failed to update and build flux install manifests: %v", err))
	}

	log.Println("Installing flux")
	if err := installFlux(ctx, kubeconfigPath, fluxInstallManifestPath); err != nil {
		log.Printf("Failed to install flux: %v", err)
	}

	exitCode = m.Run()

	// log.Println("Uninstalling flux")
	// uninstallFlux(ctx, kubeconfigPath, fluxInstallManifestPath)
}

// getProviderConfig returns the test configuration of supported providers.
func getProviderConfig(provider string) *ProviderConfig {
	switch provider {
	case "aws":
		return &ProviderConfig{
			terraformPath:      terraformPathAWS,
			registryLogin:      registryLoginECR,
			pushFluxTestImages: pushFluxTestImagesECR,
			createKubeconfig:   createKubeconfigEKS,
		}
	case "azure":
		return &ProviderConfig{
			terraformPath:      terraformPathAzure,
			registryLogin:      registryLoginACR,
			pushFluxTestImages: pushFluxTestImagesACR,
			createKubeconfig:   createKubeConfigAKS,
		}
	case "gcp":
		return &ProviderConfig{
			terraformPath:      terraformPathGCP,
			registryLogin:      registryLoginGCR,
			pushFluxTestImages: pushFluxTestImagesGCR,
			createKubeconfig:   createKubeconfigGKE,
		}
	}
	return nil
}
