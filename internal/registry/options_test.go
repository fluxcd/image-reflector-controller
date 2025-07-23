/*
Copyright 2023 The Flux authors

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

package registry_test

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/runtime/secrets"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/registry"
	"github.com/fluxcd/image-reflector-controller/internal/test"
)

func TestNewAuthOptionsGetter_GetOptions(t *testing.T) {
	testImg := "example.com/foo/bar"
	testSecretName := "test-secret"
	testTLSSecretName := "test-tls-secret"
	testDeprecatedTLSSecretName := "test-deprecated-tls-secret"
	testServiceAccountName := "test-service-account"
	testNamespace := "test-ns"

	dockerconfigjson := []byte(`
{
	"auths": {
		"example.com": {
			"username": "user",
			"password": "pass"
		}
	}
}`)

	testSecret := &corev1.Secret{}
	testSecret.Name = testSecretName
	testSecret.Namespace = testNamespace
	testSecret.Type = corev1.SecretTypeDockerConfigJson
	testSecret.Data = map[string][]byte{".dockerconfigjson": dockerconfigjson}
	g := NewWithT(t)

	// Create a test TLS server to get valid cert data. The server is never
	// started or used below.
	_, rootCertPEM, clientCertPEM, clientKeyPEM, _, err := test.CreateTLSServer()
	g.Expect(err).To(Not(HaveOccurred()))

	testTLSSecret := &corev1.Secret{}
	testTLSSecret.Name = testTLSSecretName
	testTLSSecret.Namespace = testNamespace
	testTLSSecret.Type = corev1.SecretTypeTLS
	testTLSSecret.Data = map[string][]byte{
		secrets.KeyCACert:        rootCertPEM,
		secrets.KeyTLSCert:       clientCertPEM,
		secrets.KeyTLSPrivateKey: clientKeyPEM,
	}

	testProxySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-proxy-secret",
			Namespace: testNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"address": []byte("http://proxy.example.com"),
		},
	}

	testDeprecatedTLSSecret := &corev1.Secret{}
	testDeprecatedTLSSecret.Name = testDeprecatedTLSSecretName
	testDeprecatedTLSSecret.Namespace = testNamespace
	testDeprecatedTLSSecret.Type = corev1.SecretTypeTLS
	testDeprecatedTLSSecret.Data = map[string][]byte{
		secrets.LegacyKeyCACert:        rootCertPEM,
		secrets.LegacyKeyTLSCert:       clientCertPEM,
		secrets.LegacyKeyTLSPrivateKey: clientKeyPEM,
	}

	// Docker config secret with TLS data.
	testDockerCfgSecretWithTLS := testSecret.DeepCopy()
	testDockerCfgSecretWithTLS.Data = map[string][]byte{
		secrets.KeyCACert:        rootCertPEM,
		secrets.KeyTLSCert:       clientCertPEM,
		secrets.KeyTLSPrivateKey: clientKeyPEM,
	}

	// ServiceAccount without image pull secret.
	testServiceAccount := &corev1.ServiceAccount{}
	testServiceAccount.Name = testServiceAccountName
	testServiceAccount.Namespace = testNamespace

	// ServiceAccount with image pull secret.
	testServiceAccountWithSecret := testServiceAccount.DeepCopy()
	testServiceAccountWithSecret.ImagePullSecrets = []corev1.LocalObjectReference{{Name: testSecretName}}

	tests := []struct {
		name          string
		mockObjs      []client.Object
		imageRepoSpec imagev1.ImageRepositorySpec
		wantErr       bool
	}{
		{
			name: "no auth options",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
			},
		},
		{
			name:     "secret ref with existing secret",
			mockObjs: []client.Object{testSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				SecretRef: &meta.LocalObjectReference{
					Name: testSecretName,
				},
			},
		},
		{
			name: "secret ref with non-existing secret",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				SecretRef: &meta.LocalObjectReference{
					Name: "non-existing-secret",
				},
			},
			wantErr: true,
		},
		{
			name: "contextual login",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:    "123456789000.dkr.ecr.us-east-2.amazonaws.com/test",
				Provider: "aws",
			},
			wantErr: false,
		},
		{
			name: "cloud provider repo without login",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: "123456789000.dkr.ecr.us-east-2.amazonaws.com/test",
			},
		},
		{
			name:     "cert secret ref with existing secret",
			mockObjs: []client.Object{testTLSSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: testTLSSecretName,
				},
			},
		},
		{
			name:     "cert secret ref with existing secret using deprecated keys",
			mockObjs: []client.Object{testDeprecatedTLSSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: testDeprecatedTLSSecretName,
				},
			},
		},
		{
			name: "cert secret ref with non-existing secret",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: "non-existing-secret",
				},
			},
			wantErr: true,
		},
		{
			name:     "proxy secret ref with existing secret",
			mockObjs: []client.Object{testProxySecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				ProxySecretRef: &meta.LocalObjectReference{
					Name: testProxySecret.Name,
				},
			},
		},
		{
			name: "proxy secret ref with non-existing secret",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				ProxySecretRef: &meta.LocalObjectReference{
					Name: "non-existing-secret",
				},
			},
			wantErr: true,
		},
		{
			name:     "secret, cert secret and proxy secret refs",
			mockObjs: []client.Object{testSecret, testTLSSecret, testProxySecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				SecretRef: &meta.LocalObjectReference{
					Name: testSecretName,
				},
				CertSecretRef: &meta.LocalObjectReference{
					Name: testTLSSecretName,
				},
				ProxySecretRef: &meta.LocalObjectReference{
					Name: testProxySecret.Name,
				},
			},
		},
		{
			name:     "cert secret ref of type docker config",
			mockObjs: []client.Object{testDockerCfgSecretWithTLS},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: testSecretName,
				},
			},
			wantErr: false,
		},
		{
			name:     "service account without pull secret",
			mockObjs: []client.Object{testServiceAccount},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:              testImg,
				ServiceAccountName: testServiceAccountName,
			},
		},
		{
			name:     "service account with pull secret",
			mockObjs: []client.Object{testServiceAccountWithSecret, testSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:              testImg,
				ServiceAccountName: testServiceAccountName,
			},
		},
		{
			name:     "service account with non-existing pull secret",
			mockObjs: []client.Object{testServiceAccountWithSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:              testImg,
				ServiceAccountName: testServiceAccountName,
			},
			wantErr: true,
		},
		{
			name: "unsupported provider",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:    testImg,
				Provider: "unsupported-provider",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			k8sClient := fake.NewClientBuilder().
				WithObjects(tt.mockObjs...).
				Build()
			getter := &registry.AuthOptionsGetter{Client: k8sClient}

			repo := imagev1.ImageRepository{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "reconcile-repo-",
					Generation:   1,
					Namespace:    testNamespace,
				},
				Spec: tt.imageRepoSpec,
			}

			_, err := getter.GetOptions(context.Background(), &repo, &cache.InvolvedObject{})
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func Test_ParseImageReference(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		insecure bool
		wantErr  bool
		wantRef  string
	}{
		{
			name:    "simple valid url",
			url:     "example.com/foo/bar",
			wantRef: "example.com/foo/bar",
		},
		{
			name:    "with scheme prefix",
			url:     "https://example.com/foo/bar",
			wantErr: true,
		},
		{
			name:    "with tag",
			url:     "example.com/foo/bar:baz",
			wantErr: true,
		},
		{
			name:    "with host port",
			url:     "example.com:9999/foo/bar",
			wantErr: false,
			wantRef: "example.com:9999/foo/bar",
		},
		{
			name:     "with insecure registry",
			url:      "example.com/foo/bar",
			insecure: true,
			wantRef:  "example.com/foo/bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			ref, err := registry.ParseImageReference(tt.url, tt.insecure)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ref.String()).To(Equal(tt.wantRef))
				if tt.insecure {
					g.Expect(ref.Context().Registry.Scheme()).To(Equal("http"))
				}
			}
		})
	}
}
