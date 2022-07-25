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

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	tftestenv "github.com/fluxcd/image-reflector-controller/tests/tftestenv"
)

// updateAndBuildFluxInstallManifests assumes that ./build/flux/ already exists
// with downloaded install.yaml and copied kustomization.yaml. It updates the
// kustomization.yaml with new test images and builds a new install manifest
// at ./build/flux.yaml.
func updateAndBuildFluxInstallManifests(ctx context.Context, images map[string]string) error {
	// Construct kustomize set image arguments.
	setImgArgs := []string{}
	for name, img := range images {
		// NOTE: There's an assumption here that the existing images in the
		// manifest have fluxcd/ prefixed images.
		imageName := path.Join("fluxcd", name)
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

// createAndPushImages randomly generates test images with the given tags and
// pushes them to the given test repositories.
func createAndPushImages(repos map[string]string, tags []string) error {
	// TODO: Build and push concurrently.
	for _, repo := range repos {
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
	}
	return nil
}

// retagAndPush retags local images based on the remote repo and pushes them.
func retagAndPush(ctx context.Context, repo string, localImgs map[string]string) (map[string]string, error) {
	imgs := map[string]string{}
	for name, li := range localImgs {
		remoteImage := path.Join(repo, name)
		remoteImage += ":test"

		log.Printf("pushing flux test image %s\n", remoteImage)
		// Retag local image and push.
		if err := tftestenv.RunCommand(ctx, "./",
			fmt.Sprintf("docker tag %s %s", li, remoteImage),
			tftestenv.RunCommandOptions{},
		); err != nil {
			return nil, err
		}
		if err := tftestenv.RunCommand(ctx, "./",
			fmt.Sprintf("docker push %s", remoteImage),
			tftestenv.RunCommandOptions{},
		); err != nil {
			return nil, err
		}
		imgs[name] = remoteImage
	}
	return imgs, nil
}
