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
	"fmt"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/fluxcd/test-infra/tftestenv"
)

// createKubeConfigAKS constructs kubeconfig for an AKS cluster from the
// terraform state output at the given kubeconfig path.
func createKubeConfigAKS(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	kubeconfigYaml, ok := state["aks_kubeconfig"].Value.(string)
	if !ok || kubeconfigYaml == "" {
		return fmt.Errorf("failed to obtain kubeconfig from tf output")
	}
	return tftestenv.CreateKubeconfigAKS(ctx, kubeconfigYaml, kcPath)
}

// registryLoginACR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginACR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: ACR registry accept dynamic repository creation by just pushing a
	// new image with a new repository name.
	testRepos := map[string]string{}

	registryURL := output["acr_registry_url"].Value.(string)
	fluxRegistryURL := output["flux_acr_registry_url"].Value.(string)
	if err := tftestenv.RegistryLoginACR(ctx, fluxRegistryURL); err != nil {
		return nil, err
	}

	if err := tftestenv.RegistryLoginACR(ctx, registryURL); err != nil {
		return nil, err
	}
	testRepos["acr"] = registryURL + "/" + randStringRunes(5)

	return testRepos, nil
}

// pushFluxTestImagesACR pushes flux images that are being tested. It must be
// called after registryLoginACR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func pushFluxTestImagesACR(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// Get the registry name and construct the image names accordingly.
	registryURL := output["flux_acr_registry_url"].Value.(string)
	return tftestenv.PushTestAppImagesACR(ctx, localImgs, registryURL)
}

// getKustomizePatchesAzure return the patches that should be added to the kustomization.yaml
// before deploying Flux. It returns two patches, one to annotate the image-reflector-controller
// service account and the other for the image-reflector-controller deployment. These are needed
// for workload identity to work properly on Azure
func getKustomizePatchesAzure(output map[string]*tfjson.StateOutput) []string {
	appClientId := output["spn_id"].Value.(string)
	saAnnotation := `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: image-reflector-controller
  namespace: flux-system
  annotations:
    azure.workload.identity/client-id: "%s"
  labels:
    azure.workload.identity/use: "true"
`
	saPatch := fmt.Sprintf(saAnnotation, appClientId)
	deployPatch := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: image-reflector-controller
  namespace: flux-system
  labels:
    azure.workload.identity/use: "true"
spec:
  template:
    metadata:
      labels:
        azure.workload.identity/use: "true"
	`
	return []string{deployPatch, saPatch}
}
