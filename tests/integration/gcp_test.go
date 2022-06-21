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
	"os"

	tfjson "github.com/hashicorp/terraform-json"

	tftestenv "github.com/fluxcd/image-reflector-controller/tests/tftestenv"
)

// createKubeconfigGKE constructs kubeconfig from the terraform state output at
// the given kubeconfig path.
func createKubeconfigGKE(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	kubeconfigYaml, ok := state["gcp_kubeconfig"].Value.(string)
	if !ok || kubeconfigYaml == "" {
		return fmt.Errorf("failed to obtain kubeconfig from tf output")
	}

	f, err := os.Create(kcPath)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(f, kubeconfigYaml)
	return err
	return nil
}

// registryLoginGCR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginGCR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: ECR provides pre-existing registry per account. It requires
	// repositories to be created explicitly using their API before pushing
	// image.
	repoURL := output["gcr_repository_url"].Value.(string)
	if err := tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("gcloud auth configure-docker %s", repoURL),
		tftestenv.RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	location := output["artifact_location"].Value.(string)
	project := output["artifact_project"].Value.(string)
	repository := output["artifact_repository"].Value.(string)
	artifactRegistry := fmt.Sprintf("%s-docker.pkg.dev", location)
	artifactURL := fmt.Sprintf("%s/%s/%s", artifactRegistry, project, repository)
	if err := tftestenv.RunCommand(ctx, "./",
		fmt.Sprintf("gcloud auth configure-docker %s", artifactRegistry),
		tftestenv.RunCommandOptions{},
	); err != nil {
		return nil, err
	}

	return map[string]string{
		"gcr":             repoURL + "/" + randStringRunes(5),
		"artifact_regist": artifactURL + "/" + randStringRunes(5),
	}, nil
}
