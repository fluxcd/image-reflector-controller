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
	"log"
	"os"

	tfjson "github.com/hashicorp/terraform-json"

	tftestenv "github.com/fluxcd/image-reflector-controller/tests/tftestenv"
)

// Based on https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
const kubeConfigTmpl = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %[1]s
    server: %[2]s
  name: %[3]s
contexts:
- context:
    cluster: %[3]s
    user: %[4]s
  name: %[3]s
current-context: %[3]s
kind: Config
preferences: {}
users:
- name: %[4]s
  user:
    token: %[5]s
`

// kubeconfigWithClusterAuthToken returns a kubeconfig with the given cluster
// authentication token.
func kubeconfigWithClusterAuthToken(token, caData, endpoint, user, clusterName string) string {
	return fmt.Sprintf(kubeConfigTmpl, caData, endpoint, clusterName, user, token)
}

// getEKSClientToken fetches the EKS cluster client token.
func getEKSClientToken(ctx context.Context, clusterName string) ([]byte, error) {
	err := tftestenv.RunCommand(ctx, "build",
		fmt.Sprintf("aws eks get-token --cluster-name %s | jq -r .status.token > token", clusterName),
		tftestenv.RunCommandOptions{},
	)
	if err != nil {
		return nil, err
	}
	return os.ReadFile("build/token")
}

// createKubeconfigEKS constructs kubeconfig from the terraform state output at
// the given kubeconfig path.
func createKubeconfigEKS(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	clusterName := state["eks_cluster_name"].Value.(string)
	eksHost := state["eks_cluster_endpoint"].Value.(string)
	eksClusterArn := state["eks_cluster_arn"].Value.(string)
	eksCa := state["eks_cluster_ca_certificate"].Value.(string)
	eksToken, err := getEKSClientToken(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to obtain auth token: %w", err)
	}

	kubeconfigYaml := kubeconfigWithClusterAuthToken(string(eksToken), eksCa, eksHost, eksClusterArn, clusterName)

	f, err := os.Create(kcPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(f, kubeconfigYaml)
	return err
}

// registryLoginECR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginECR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: ECR provides pre-existing registry per account. It requires
	// repositories to be created explicitly using their API before pushing
	// image.
	testRepoURL := output["ecr_repository_url"].Value.(string)
	ircRepoURL := output["ecr_image_reflector_controller_repo_url"].Value.(string)
	region := output["region"].Value.(string)

	if err := tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("aws ecr get-login-password --region %s | docker login --username AWS --password-stdin %s", region, testRepoURL),
		tftestenv.RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	if err := tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("aws ecr get-login-password --region %s | docker login --username AWS --password-stdin %s", region, ircRepoURL),
		tftestenv.RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	return map[string]string{"ecr": testRepoURL}, nil
}

// pushFluxTestImagesECR pushes flux image that is being tested. It must be
// called after registryLoginECR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func pushFluxTestImagesECR(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: Unlike Azure Container Registry and Google Artifact Registry, ECR
	// does not support dynamic image repositories. A new repository for a new
	// image has to be explicitly created. Therefore, the single local image
	// is retagged and pushed in the already created repository.
	if len(localImgs) != 1 {
		return nil, fmt.Errorf("ECR repository supports pushing one image only, got: %v", localImgs)
	}

	// Get the registry name and construct the image names accordingly.
	repo := output["ecr_image_reflector_controller_repo_url"].Value.(string)

	remoteImage := repo + ":test"

	// Extract the component name and local image.
	var name, localImage string
	for n, i := range localImgs {
		name, localImage = n, i
	}

	if err := tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("docker tag %s %s", localImage, remoteImage),
		tftestenv.RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	log.Printf("pushing flux test image %s\n", remoteImage)

	if err := tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("docker push %s", remoteImage),
		tftestenv.RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	return map[string]string{
		name: remoteImage,
	}, nil
}
