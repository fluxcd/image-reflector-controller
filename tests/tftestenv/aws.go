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

	tfjson "github.com/hashicorp/terraform-json"
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

func getClientToken(ctx context.Context, clusterName string) ([]byte, error) {
	err := RunCommand(ctx, "build",
		fmt.Sprintf("aws eks get-token --cluster-name %s | jq -r .status.token > token", clusterName),
		RunCommandOptions{},
	)
	if err != nil {
		return nil, err
	}
	return os.ReadFile("build/token")
}

// CreateEKSKubeconfig constructs kubeconfig for an EKS cluster from the terraform state output at the
// given kubeconfig path.
func CreateEKSKubeconfig(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	clusterName := state["eks_cluster_name"].Value.(string)
	eksHost := state["eks_cluster_endpoint"].Value.(string)
	eksClusterArn := state["eks_cluster_endpoint"].Value.(string)
	eksCa := state["eks_cluster_ca_certificate"].Value.(string)
	eksToken, err := getClientToken(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to obtain auth token: %w", err)
	}

	kubeconfigYaml := kubeconfigWithClusterAuthToken(string(eksToken), eksCa, eksHost, eksClusterArn, eksClusterArn)

	f, err := os.Create(kcPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(f, kubeconfigYaml)
	return err
}
