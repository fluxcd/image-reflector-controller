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

package secret

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
)

func TestExtractAuthn(t *testing.T) {
	// the secret in testdata/secret.json was created with kubectl
	// create secret docker-registry. Test that it can be decoded to
	// get an authentication value.
	b, err := os.ReadFile("testdata/secret.json")
	if err != nil {
		t.Fatal(err)
	}
	var secret corev1.Secret
	if err = json.Unmarshal(b, &secret); err != nil {
		t.Fatal(err)
	}
	dockerReg, err := name.ParseReference("docker.io/stefan/podinfo:v5.1.02")
	if err != nil {
		t.Fatal(err)
	}

	auth, err := AuthFromSecret(secret, dockerReg)
	if err != nil {
		t.Fatal(err)
	}
	authConfig, err := auth.Authorization()
	if err != nil {
		t.Fatal()
	}
	if authConfig.Username != "fooser" || authConfig.Password != "foopass" {
		t.Errorf("expected username/password to be fooser/foopass, got %s/%s",
			authConfig.Username, authConfig.Password)
	}
}

func TestExtractAuthForURLs(t *testing.T) {
	dockerReg, err := name.ParseReference("docker.io/stefan/podinfo:5.1.2")
	if err != nil {
		t.Fatal(err)
	}

	portReg, err := name.ParseReference("registry.me:8082/stefan/podinfo:v5.1.02")
	if err != nil {
		t.Fatal(err)
	}

	testFiles := []struct {
		secretFile string
		registry   name.Reference
	}{
		{
			secretFile: "secret.json",
			registry:   dockerReg,
		},
		{
			secretFile: "auth_secret_with_http.json",
			registry:   dockerReg,
		},
		{
			secretFile: "auth_secret_without_https.json",
			registry:   dockerReg,
		},
		{
			secretFile: "auth_secret_with_port_without_https.json",
			registry:   portReg,
		},
		{
			secretFile: "auth_secret_with_http_and_port.json",
			registry:   portReg,
		},
	}

	for _, test := range testFiles {
		b, err := os.ReadFile("testdata/" + test.secretFile)
		if err != nil {
			t.Fatal(err)
		}
		var secret corev1.Secret
		if err = json.Unmarshal(b, &secret); err != nil {
			t.Fatal(err)
		}

		_, err = AuthFromSecret(secret, test.registry)
		if err != nil {
			t.Fatalf("error getting secret for %s: %s", "index.docker.io", err)
		}
	}
}
