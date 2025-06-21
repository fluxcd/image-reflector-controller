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

package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/fluxcd/pkg/cache"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/secret"
)

// AuthOptionsGetter builds a slice of options from an ImageRepository by looking up references to Secrets etc.
// on the Kubernetes cluster using the provided client interface. If no external authentication provider is
// configured on the ImageRepository, the given ProviderOptions are used for authentication. Options are extracted
// from the following ImageRepository spec fields:
//
// - spec.image
// - spec.secretRef
// - spec.provider
// - spec.certSecretRef
// - spec.serviceAccountName
type AuthOptionsGetter struct {
	client.Client
	TokenCache *cache.TokenCache
}

func (r *AuthOptionsGetter) GetOptions(ctx context.Context, repo *imagev1.ImageRepository,
	involvedObject *cache.InvolvedObject) ([]remote.Option, error) {
	timeout := repo.GetTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var transportOptions []func(*http.Transport)

	// Load proxy configuration.
	var proxyURL *url.URL
	var err error
	if repo.Spec.ProxySecretRef != nil {
		proxyURL, err = r.GetProxyURL(ctx, repo)
		if err != nil {
			return nil, err
		}
		if proxyURL != nil {
			transportOptions = append(transportOptions, func(t *http.Transport) {
				t.Proxy = http.ProxyURL(proxyURL)
			})
		}
	}

	// Configure authentication strategy to access the registry.
	var options []remote.Option
	var authSecret corev1.Secret
	var authenticator authn.Authenticator
	var authErr error

	if repo.Spec.SecretRef != nil {
		ref, err := ParseImageReference(repo.Spec.Image, repo.Spec.Insecure)
		if err != nil {
			return nil, fmt.Errorf("failed parsing image reference %q: %w", repo.Spec.Image, err)
		}

		if err := r.Get(ctx, types.NamespacedName{
			Namespace: repo.GetNamespace(),
			Name:      repo.Spec.SecretRef.Name,
		}, &authSecret); err != nil {
			return nil, err
		}
		authenticator, authErr = secret.AuthFromSecret(authSecret, ref)
	} else if provider := repo.GetProvider(); provider != "generic" {
		// Build login provider options and use it to attempt registry login.
		var opts []auth.Option
		if proxyURL != nil {
			opts = append(opts, auth.WithProxyURL(*proxyURL))
		}
		if repo.Spec.ServiceAccountName != "" {
			serviceAccount := client.ObjectKey{
				Name:      repo.Spec.ServiceAccountName,
				Namespace: repo.GetNamespace(),
			}
			opts = append(opts, auth.WithServiceAccount(serviceAccount, r.Client))
		}
		if r.TokenCache != nil {
			opts = append(opts, auth.WithCache(*r.TokenCache, *involvedObject))
		}
		authenticator, authErr = authutils.GetArtifactRegistryCredentials(ctx, provider, repo.Spec.Image, opts...)
	}
	if authErr != nil {
		return nil, authErr
	}
	if authenticator != nil {
		options = append(options, remote.WithAuth(authenticator))
	}

	// Load any provided certificate.
	if repo.Spec.CertSecretRef != nil {
		var certSecret corev1.Secret
		if repo.Spec.SecretRef != nil && repo.Spec.SecretRef.Name == repo.Spec.CertSecretRef.Name {
			certSecret = authSecret
		} else {
			if err := r.Get(ctx, types.NamespacedName{
				Namespace: repo.GetNamespace(),
				Name:      repo.Spec.CertSecretRef.Name,
			}, &certSecret); err != nil {
				return nil, err
			}
		}

		tlsConfig, err := secret.TLSConfigFromKubeTLSSecret(&certSecret)
		if err != nil {
			return nil, err
		}
		if tlsConfig == nil {
			tlsConfig, err = secret.TLSConfigFromSecret(&certSecret)
			if err != nil {
				return nil, err
			}
			if tlsConfig != nil {
				ctrl.LoggerFrom(ctx).
					Info("warning: specifying TLS auth data via `certFile`/`keyFile`/`caFile` is deprecated, please use `tls.crt`/`tls.key`/`ca.crt` instead")
			}
		}
		if tlsConfig != nil {
			transportOptions = append(transportOptions, func(t *http.Transport) {
				t.TLSClientConfig = tlsConfig
			})
		}
	}

	// Specify any transport options.
	if len(transportOptions) > 0 {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		for _, opt := range transportOptions {
			opt(tr)
		}
		options = append(options, remote.WithTransport(tr))
	}

	if authenticator == nil && repo.Spec.ServiceAccountName != "" {
		serviceAccount := corev1.ServiceAccount{}
		// Lookup service account
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: repo.GetNamespace(),
			Name:      repo.Spec.ServiceAccountName,
		}, &serviceAccount); err != nil {
			return nil, err
		}

		if len(serviceAccount.ImagePullSecrets) > 0 {
			imagePullSecrets := make([]corev1.Secret, len(serviceAccount.ImagePullSecrets))
			for i, ips := range serviceAccount.ImagePullSecrets {
				var saAuthSecret corev1.Secret
				if err := r.Get(ctx, types.NamespacedName{
					Namespace: repo.GetNamespace(),
					Name:      ips.Name,
				}, &saAuthSecret); err != nil {
					return nil, err
				}
				imagePullSecrets[i] = saAuthSecret
			}
			keychain, err := k8schain.NewFromPullSecrets(ctx, imagePullSecrets)
			if err != nil {
				return nil, err
			}
			options = append(options, remote.WithAuthFromKeychain(keychain))
		}
	}

	return options, nil
}

// GetProxyURL gets the proxy configuration for the transport based on the
// specified proxy secret reference in the ImageRepository object.
func (r *AuthOptionsGetter) GetProxyURL(ctx context.Context, obj *imagev1.ImageRepository) (*url.URL, error) {
	if obj.Spec.ProxySecretRef == nil || obj.Spec.ProxySecretRef.Name == "" {
		return nil, nil
	}

	proxySecretName := types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Spec.ProxySecretRef.Name,
	}
	var proxySecret corev1.Secret
	if err := r.Get(ctx, proxySecretName, &proxySecret); err != nil {
		return nil, err
	}

	proxyData := proxySecret.Data
	address, ok := proxyData["address"]
	if !ok {
		return nil, fmt.Errorf("invalid proxy secret '%s/%s': key 'address' is missing",
			obj.Namespace, obj.Spec.ProxySecretRef.Name)
	}
	proxyURL, err := url.Parse(string(address))
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy address '%s': %w", address, err)
	}
	user, hasUser := proxyData["username"]
	password, hasPassword := proxyData["password"]
	if hasUser || hasPassword {
		proxyURL.User = url.UserPassword(string(user), string(password))
	}
	return proxyURL, nil
}
