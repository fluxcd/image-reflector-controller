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

// createKubeconfigGKE constructs kubeconfig from the terraform state output at
// the given kubeconfig path.
func createKubeconfigGKE(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	kubeconfigYaml, ok := state["gcp_kubeconfig"].Value.(string)
	if !ok || kubeconfigYaml == "" {
		return fmt.Errorf("failed to obtain kubeconfig from tf output")
	}
	return tftestenv.CreateKubeconfigGKE(ctx, kubeconfigYaml, kcPath)
}

// registryLoginGCR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginGCR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: GCR accepts dynamic repository creation by just pushing a new image
	// with a new repository name.
	testRepos := map[string]string{}

	project := output["gcp_project"].Value.(string)
	region := output["gcp_region"].Value.(string)
	repositoryID := output["gcp_artifact_repository"].Value.(string)
	artifactRegistryURL, artifactRepoURL := tftestenv.GetGoogleArtifactRegistryAndRepository(project, region, repositoryID)
	if err := tftestenv.RegistryLoginGCR(ctx, artifactRegistryURL); err != nil {
		return nil, err
	}
	testRepos["artifact_registry"] = artifactRepoURL + "/" + randStringRunes(5)

	return testRepos, nil
}

// pushFluxTestImagesGCR pushes flux images that are being tested. It must be
// called after registryLoginGCR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func pushFluxTestImagesGCR(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	project := output["gcp_project"].Value.(string)
	region := output["gcp_region"].Value.(string)
	repositoryID := output["gcp_artifact_repository"].Value.(string)
	return tftestenv.PushTestAppImagesGCR(ctx, localImgs, project, region, repositoryID)
}
