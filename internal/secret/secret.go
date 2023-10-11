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

const (
	ClientCert = "certFile"
	ClientKey  = "keyFile"
	CACert     = "caFile"
	CACrtKey   = "ca.crt"
)

type dockerConfig struct {
	Auths map[string]authn.AuthConfig
}

// TransportFromSecret reads the TLS data specified in the provided Secret
// and returns a transport configured with the appropriate TLS settings.
// It checks for the following keys in the Secret:
// - `caFile`,  for the CA certificate
// - `certFile` and `keyFile`, for the certificate and private key
//
// If none of these keys exists in the Secret then an empty transport is
// returned. If only a certificate OR private key is found, an error is
// returned.
func TransportFromSecret(certSecret *corev1.Secret) (*http.Transport, error) {
	// It's possible the secret doesn't contain any certs after
	// all and the default transport could be used; but it's
	// simpler here to assume a fresh transport is needed.
	transport := &http.Transport{}
	config, err := tlsConfigFromSecret(certSecret, false)
	if err != nil {
		return nil, err
	}
	if config != nil {
		transport.TLSClientConfig = config
	}

	return transport, nil
}

// TransportFromKubeTLSSecret reads the TLS data specified in the provided
// Secret and returns a transport configured with the appropriate TLS settings.
// It checks for the following keys in the Secret:
// - `ca.crt`,  for the CA certificate
// - `tls.crt` and `tls.key`, for the certificate and private key
//
// If none of these keys exists in the Secret then an empty transport is
// returned. If only a certificate OR private key is found, an error is
// returned.
func TransportFromKubeTLSSecret(certSecret *corev1.Secret) (*http.Transport, error) {
	// It's possible the secret doesn't contain any certs after
	// all and the default transport could be used; but it's
	// simpler here to assume a fresh transport is needed.
	transport := &http.Transport{}
	config, err := tlsConfigFromSecret(certSecret, true)
	if err != nil {
		return nil, err
	}
	if config != nil {
		transport.TLSClientConfig = config
	}

	return transport, nil
}

// tlsClientConfigFromSecret attempts to construct and return a TLS client
// config from the given Secret. If the Secret does not contain any TLS
// data, it returns nil.
//
// kubernetesTLSKeys is a boolean indicating whether to check the Secret
// for keys expected to be present in a Kubernetes TLS Secret. Based on its
// value, the Secret is checked for the following keys:
// - tls.key/keyFile for the private key
// - tls.crt/certFile for the certificate
// - ca.crt/caFile for the CA certificate
// The keys should adhere to a single convention, i.e. a Secret with tls.key
// and certFile is invalid.
// Copied from: https://github.com/fluxcd/source-controller/blob/052221c3d8a3ce5fd1a1328db4cc27d31bfd5e59/internal/tls/config.go#L78
func tlsConfigFromSecret(secret *corev1.Secret, kubernetesTLSKeys bool) (*tls.Config, error) {
	// Only Secrets of type Opaque and TLS are allowed. We also allow Secrets with a blank
	// type, to avoid having to specify the type of the Secret for every test case.
	// Since a real Kubernetes Secret is of type Opaque by default, its safe to allow this.
	switch secret.Type {
	case corev1.SecretTypeOpaque, corev1.SecretTypeTLS, "":
	default:
		return nil, fmt.Errorf("cannot use secret '%s' to construct TLS config: invalid secret type: '%s'", secret.Name, secret.Type)
	}

	var certBytes, keyBytes, caBytes []byte
	if kubernetesTLSKeys {
		certBytes, keyBytes, caBytes = secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], secret.Data[CACrtKey]
	} else {
		certBytes, keyBytes, caBytes = secret.Data[ClientCert], secret.Data[ClientKey], secret.Data[CACert]
	}

	switch {
	case len(certBytes)+len(keyBytes)+len(caBytes) == 0:
		return nil, nil
	case (len(certBytes) > 0 && len(keyBytes) == 0) || (len(keyBytes) > 0 && len(certBytes) == 0):
		return nil, fmt.Errorf("invalid '%s' secret data: both certificate and private key need to be provided",
			secret.Name)
	}

	tlsConf := &tls.Config{}
	if len(certBytes) > 0 && len(keyBytes) > 0 {
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return nil, err
		}
		tlsConf.Certificates = append(tlsConf.Certificates, cert)
	}

	if len(caBytes) > 0 {
		cp, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve system certificate pool: %w", err)
		}
		if !cp.AppendCertsFromPEM(caBytes) {
			return nil, fmt.Errorf("cannot append certificate into certificate pool: invalid CA certificate")
		}

		tlsConf.RootCAs = cp
	}

	return tlsConf, nil

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
		return "", errors.New("empty url")
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
		return "", fmt.Errorf(
			"expected an HTTPS URL instead of '%s' (e.g. 'https://index.docker.io/v2/' or 'https://index.docker.io'), or the same without 'https://' (e.g., 'index.docker.io/v2/' or 'index.docker.io')",
			urlStr)
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
