/*
Copyright 2020 The Flux authors

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

package controllers

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestExtractAuthn(t *testing.T) {
	// the secret in testdata/secret.json was created with kubectl
	// create secret docker-registry. Test that it can be decoded to
	// get an authentication value.
	b, err := ioutil.ReadFile("testdata/secret.json")
	if err != nil {
		t.Fatal(err)
	}
	var secret corev1.Secret
	if err = json.Unmarshal(b, &secret); err != nil {
		t.Fatal(err)
	}
	auth, err := authFromSecret(secret, "https://index.docker.io/v1/")
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
