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

// CreateKubeconfigAKS constructs kubeconfig for an AKS cluster from the
// terraform state output at the given kubeconfig path.
func CreateKubeconfigAKS(ctx context.Context, kubeconfigYaml string, kcPath string) error {
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

// RegistryLoginACR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func RegistryLoginACR(ctx context.Context, registryURL string) error {
	return RunCommand(ctx, "./",
		fmt.Sprintf("az acr login --name %s", registryURL),
		RunCommandOptions{},
	)
}

// PushTestAppImagesACR pushes app images that are being tested. It must be
// called after RegistryLoginACR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func PushTestAppImagesACR(ctx context.Context, localImgs map[string]string, registryURL string) (map[string]string, error) {
	return retagAndPush(ctx, registryURL, localImgs)
}
