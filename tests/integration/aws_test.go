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

	tfjson "github.com/hashicorp/terraform-json"

	tftestenv "github.com/fluxcd/image-reflector-controller/tests/tftestenv"
)

// createKubeconfigEKS constructs kubeconfig from the terraform state output at
// the given kubeconfig path.
func createKubeconfigEKS(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	clusterName := state["eks_cluster_name"].Value.(string)
	eksHost := state["eks_cluster_endpoint"].Value.(string)
	eksClusterArn := state["eks_cluster_arn"].Value.(string)
	eksCa := state["eks_cluster_ca_certificate"].Value.(string)
	return tftestenv.CreateKubeconfigEKS(ctx, clusterName, eksHost, eksClusterArn, eksCa, kcPath)
}

// registryLoginECR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginECR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: ECR provides pre-existing registry per account. It requires
	// repositories to be created explicitly using their API before pushing
	// image.
	testRepos := map[string]string{}
	region := output["region"].Value.(string)

	testRepoURL := output["ecr_repository_url"].Value.(string)
	if err := tftestenv.RegistryLoginECR(ctx, region, testRepoURL); err != nil {
		return nil, err
	}
	testRepos["ecr"] = testRepoURL

	// Log into the image-reflector-controller repository to be able to push to
	// it. This image is not used in testing and need not be included in
	// testRepos.
	ircRepoURL := output["ecr_image_reflector_controller_repo_url"].Value.(string)
	if err := tftestenv.RegistryLoginECR(ctx, region, ircRepoURL); err != nil {
		return nil, err
	}

	return testRepos, nil
}

// pushFluxTestImagesECR pushes flux image that is being tested. It must be
// called after registryLoginECR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func pushFluxTestImagesECR(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// Get the registry name and construct the image names accordingly.
	repo := output["ecr_image_reflector_controller_repo_url"].Value.(string)
	remoteImage := repo + ":test"
	return tftestenv.PushTestAppImagesECR(ctx, localImgs, remoteImage)
}
