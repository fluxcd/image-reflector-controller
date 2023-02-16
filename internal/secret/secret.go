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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// These are intended to match the keys used in e.g.,
// https://github.com/fluxcd/flux2/blob/main/cmd/flux/create_secret_helm.go,
// for consistency (and perhaps this will have its own flux create
// secret subcommand at some point).
const (
	ClientCert = "certFile"
	ClientKey  = "keyFile"
	CACert     = "caFile"
)

type dockerConfig struct {
	Auths map[string]authn.AuthConfig
}

func TransportFromSecret(certSecret *corev1.Secret) (*http.Transport, error) {
	// It's possible the secret doesn't contain any certs after
	// all and the default transport could be used; but it's
	// simpler here to assume a fresh transport is needed.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	tlsConfig := transport.TLSClientConfig

	if clientCert, ok := certSecret.Data[ClientCert]; ok {
		// parse and set client cert and secret
		if clientKey, ok := certSecret.Data[ClientKey]; ok {
			cert, err := tls.X509KeyPair(clientCert, clientKey)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, cert)
		} else {
			return nil, fmt.Errorf("client certificate found, but no key")
		}
	}
	if caCert, ok := certSecret.Data[CACert]; ok {
		syscerts, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		syscerts.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = syscerts
	}

	return transport, nil
}

// authFromSecret creates an Authenticator that can be given to the
// `remote` funcs, from a Kubernetes secret. If the secret doesn't
// have the right format or data, it returns an error.
func AuthFromSecret(secret corev1.Secret, ref name.Reference) (authn.Authenticator, error) {
	switch secret.Type {
	case "kubernetes.io/dockerconfigjson":
		var dockerconfig dockerConfig
		configData := secret.Data[".dockerconfigjson"]
		if err := json.NewDecoder(bytes.NewBuffer(configData)).Decode(&dockerconfig); err != nil {
			return nil, err
		}

		authMap, err := parseAuthMap(dockerconfig)
		if err != nil {
			return nil, err
		}
		registry := ref.Context().RegistryStr()
		auth, ok := authMap[registry]
		if !ok {
			return nil, fmt.Errorf("auth for %q not found in secret %v", registry, types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()})
		}
		return authn.FromConfig(auth), nil
	default:
		return nil, fmt.Errorf("unknown secret type %q", secret.Type)
	}
}

func parseAuthMap(config dockerConfig) (map[string]authn.AuthConfig, error) {
	auth := map[string]authn.AuthConfig{}
	for url, entry := range config.Auths {
		host, err := getURLHost(url)
		if err != nil {
			return nil, err
		}

		auth[host] = entry
	}

	return auth, nil
}

func getURLHost(urlStr string) (string, error) {
	if urlStr == "http://" || urlStr == "https://" {
		return "", errors.New("Empty url")
	}

	// ensure url has https:// or http:// prefix
	// url.Parse won't parse the ip:port format very well without the prefix.
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = fmt.Sprintf("https://%s/", urlStr)
	}

	// Some users were passing in credentials in the form of
	// http://docker.io and http://docker.io/v1/, etc.
	// So strip everything down to the host.
	// Also, the registry might be local and on a different port.
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	if u.Host == "" {
		return "", errors.New(fmt.Sprintf(
			"Invalid registry auth key: %s. Expected an HTTPS URL (e.g. 'https://index.docker.io/v2/' or 'https://index.docker.io'), or the same without the 'https://' (e.g., 'index.docker.io/v2/' or 'index.docker.io')",
			urlStr))
	}

	return u.Host, nil
}

func Fuzz_imagerepository_getURLHost(f *testing.F) {
	f.Add("http://test")
	f.Add("http://")
	f.Add("http:///")
	f.Add("test")
	f.Add(" ")

	f.Fuzz(func(t *testing.T, url string) {
		_, _ = getURLHost(url)
	})
}
