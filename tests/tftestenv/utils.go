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
	"os/exec"
	"path"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// RunCommandOptions is used to configure the RunCommand execution.
type RunCommandOptions struct {
	Shell   string
	EnvVars []string
}

// RunCommand executes given command in a given directory.
func RunCommand(ctx context.Context, dir, command string, opts RunCommandOptions) error {
	shell := "bash"

	if opts.Shell != "" {
		shell = opts.Shell
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, shell, "-c", command)
	cmd.Dir = dir
	// Add env vars.
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, opts.EnvVars...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run command %s: %v", string(output), err)
	}
	return nil
}

// CreateAndPushImages randomly generates test images with the given tags and
// pushes them to the given test repositories.
func CreateAndPushImages(repos map[string]string, tags []string) error {
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

// retagAndPush retags local images based on the remote repo and pushes them
// with :test tag.
func retagAndPush(ctx context.Context, registry string, localImgs map[string]string) (map[string]string, error) {
	imgs := map[string]string{}
	for name, li := range localImgs {
		remoteImage := path.Join(registry, name)
		remoteImage += ":test"

		log.Printf("pushing flux test image %s\n", remoteImage)
		// Retag local image and push.
		if err := RunCommand(ctx, "./",
			fmt.Sprintf("docker tag %s %s", li, remoteImage),
			RunCommandOptions{},
		); err != nil {
			return nil, err
		}
		if err := RunCommand(ctx, "./",
			fmt.Sprintf("docker push %s", remoteImage),
			RunCommandOptions{},
		); err != nil {
			return nil, err
		}
		imgs[name] = remoteImage
	}
	return imgs, nil
}
