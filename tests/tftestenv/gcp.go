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
	"os"
)

// CreateKubeconfigGKE constructs kubeconfig from the terraform state output at
// the given kubeconfig path.
func CreateKubeconfigGKE(ctx context.Context, kubeconfigYaml string, kcPath string) error {
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

// RegistryLoginGCR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
// func registryLoginGCR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
func RegistryLoginGCR(ctx context.Context, repoURL string) error {
	return RunCommand(ctx, "./",
		fmt.Sprintf("gcloud auth configure-docker %s", repoURL),
		RunCommandOptions{},
	)
}

// GetGoogleArtifactRegistryAndRepository returns artifact registry and artifact
// repository URL from the given repository ID.
func GetGoogleArtifactRegistryAndRepository(project, region, repositoryID string) (string, string) {
	// NOTE: Artifact Registry calls a registry a "repository". A repository can
	// contain multiple different images, unlike ECR or ACR where a repository
	// can contain multiple tags of only a single image.
	// Artifact Registry also supports dynamic repository(image) creation by
	// pushing a new image with a new image name once a new registry(repository)
	// is created.

	// Use the fixed docker formatted repository suffix with the region to
	// create the registry address.
	artifactRegistry := fmt.Sprintf("%s-docker.pkg.dev", region)
	artifactRepository := fmt.Sprintf("%s/%s/%s", artifactRegistry, project, repositoryID)
	return artifactRegistry, artifactRepository
}

// PushTestAppImagesGCR pushes app images that are being tested. It must be
// called after RegistryLoginGCR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func PushTestAppImagesGCR(ctx context.Context, localImgs map[string]string, project, region, artifactRepoID string) (map[string]string, error) {
	// Get the repository name and construct the image names accordingly.
	_, repo := GetGoogleArtifactRegistryAndRepository(project, region, artifactRepoID)
	return retagAndPush(ctx, repo, localImgs)
}
