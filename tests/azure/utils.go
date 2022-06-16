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
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

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

func installFlux(ctx context.Context, kubeconfig, installManifest string) error {
	return tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("kubectl --kubeconfig=%s apply -f %s", kubeconfig, installManifest),
		tftestenv.RunCommandOptions{},
	)
}

func uninstallFlux(ctx context.Context, kubeconfig, installManifest string) error {
	return tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("kubectl --kubeconfig=%s delete -f %s", kubeconfig, installManifest),
		tftestenv.RunCommandOptions{},
	)
}

func registryLogin(ctx context.Context, registry string) error {
	return tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("az acr login --name %s  --expose-token --output tsv --query accessToken | docker login --username 00000000-0000-0000-0000-000000000000 --password-stdin %s",
			registry, registry),
		tftestenv.RunCommandOptions{},
	)
}

// func getClientToken(ctx context.Context, clusterName string) ([]byte, error) {
// 	err := tftestenv.RunCommand(ctx, "build",
// 		fmt.Sprintf("aws eks get-token --cluster-name %s | jq -r .status.token > token", clusterName),
// 		tftestenv.RunCommandOptions{},
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return os.ReadFile("build/token")
// }

// createAndPushImages randomly generates test images with the given tags and
// pushes them to the given repository.
func createAndPushImages(repo string, tags []string) error {
	// TODO: Build and push concurrently.
	for _, tag := range tags {
		imgRef := repo + ":" + tag
		ref, err := name.ParseReference(imgRef)
		if err != nil {
			return err
		}

		// Use the login credentials from the host docker/podman client config.
		opts := []remote.Option{
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
		}

		// Create a random image.
		img, err := random.Image(1024, 1)
		if err != nil {
			return err
		}

		log.Printf("pushing test image %s\n", ref.String())
		if err := remote.Write(ref, img, opts...); err != nil {
			return err
		}
	}
	return nil
}
