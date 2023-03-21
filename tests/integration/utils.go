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
	"path"
	"strings"

	"github.com/fluxcd/test-infra/tftestenv"
)

// updateAndBuildFluxInstallManifests assumes that ./build/flux/ already exists
// with downloaded install.yaml and copied kustomization.yaml. It updates the
// kustomization.yaml with new test images and patchkes. Then it builds a new install manifest
// at ./build/flux.yaml.
func updateAndBuildFluxInstallManifests(ctx context.Context, images map[string]string, patches []string) error {
	// Construct kustomize set image arguments.
	setImgArgs := []string{}
	for name, img := range images {
		// NOTE: There's an assumption here that the existing images in the
		// manifest have ghcr.io/fluxcd/ prefixed images.
		imageName := path.Join("ghcr.io/fluxcd", name)
		arg := fmt.Sprintf("%s=%s", imageName, img)
		setImgArgs = append(setImgArgs, arg)
	}

	log.Println("setting images:", setImgArgs)
	// Update all the images in kustomization.
	err := tftestenv.RunCommand(ctx, "./build/flux",
		fmt.Sprintf("kustomize edit set image %s", strings.Join(setImgArgs, " ")),
		tftestenv.RunCommandOptions{},
	)
	if err != nil {
		return err
	}

	for _, patch := range patches {
		// add patches to kustomization.
		err := tftestenv.RunCommand(ctx, "./build/flux",
			fmt.Sprintf("kustomize edit add patch --patch '%s'", patch),
			tftestenv.RunCommandOptions{},
		)
		if err != nil {
			return err
		}
	}

	// Build install manifest.
	err = tftestenv.RunCommand(ctx, "./",
		"kustomize build build/flux > build/flux.yaml",
		tftestenv.RunCommandOptions{},
	)
	if err != nil {
		return err
	}

	return nil
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
