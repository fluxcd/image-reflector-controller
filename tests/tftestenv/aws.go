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

package tftestenv

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// getEKSClientToken fetches the EKS cluster client token and writes into
// workdir/token.
func getEKSClientToken(ctx context.Context, tokenPath string, clusterName string) ([]byte, error) {
	err := RunCommand(ctx, "./",
		fmt.Sprintf("aws eks get-token --cluster-name %s | jq -r .status.token > %s", clusterName, tokenPath),
		RunCommandOptions{},
	)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(tokenPath)
}

// CreateKubeconfigEKS constructs kubeconfig from the terraform state output at
// the given kubeconfig path.
// func createKubeconfigEKS(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
func CreateKubeconfigEKS(ctx context.Context, clusterName, eksHost, eksClusterArn, eksCa, kcPath string) error {
	// Write the token next to the kubeconfig.
	// If kcPath is build/kubeconfig, tokenPath can be build/token.
	tokenPath := filepath.Join(filepath.Dir(kcPath), "token")
	eksToken, err := getEKSClientToken(ctx, tokenPath, clusterName)
	if err != nil {
		return fmt.Errorf("failed to obtain auth token: %w", err)
	}

	kubeconfigYaml := kubeconfigWithClusterAuthToken(string(eksToken), eksCa, eksHost, eksClusterArn, clusterName)

	f, err := os.Create(kcPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(f, kubeconfigYaml)
	if err != nil {
		return err
	}
	return f.Close()
}

// RegistryLoginECR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func RegistryLoginECR(ctx context.Context, region, repoURL string) error {
	return RunCommand(ctx, "./",
		fmt.Sprintf("aws ecr get-login-password --region %s | docker login --username AWS --password-stdin %s", region, repoURL),
		RunCommandOptions{},
	)
}

// PushTestAppImagesECR pushes app image that is being tested. It must be
// called after RegistryLoginECR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func PushTestAppImagesECR(ctx context.Context, localImgs map[string]string, remoteImage string) (map[string]string, error) {
	// NOTE: Unlike Azure Container Registry and Google Artifact Registry, ECR
	// does not support dynamic image repositories. A new repository for a new
	// image has to be explicitly created. Therefore, the single local image
	// is retagged and pushed in the already created repository.
	if len(localImgs) != 1 {
		return nil, fmt.Errorf("ECR repository supports pushing one image only, got: %v", localImgs)
	}

	// Extract the component name and local image.
	var name, localImage string
	for n, i := range localImgs {
		name, localImage = n, i
	}

	if err := RunCommand(ctx, "./",
		fmt.Sprintf("docker tag %s %s", localImage, remoteImage),
		RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	log.Printf("pushing flux test image %s\n", remoteImage)

	if err := RunCommand(ctx, "./",
		fmt.Sprintf("docker push %s", remoteImage),
		RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	return map[string]string{
		name: remoteImage,
	}, nil
}
