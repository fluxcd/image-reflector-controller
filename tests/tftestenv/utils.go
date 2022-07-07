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
	"os/exec"
	"time"
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
