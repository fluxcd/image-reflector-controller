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
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/runtime/secrets"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
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
		proxySecretRef := types.NamespacedName{
			Name:      repo.Spec.ProxySecretRef.Name,
			Namespace: repo.Namespace,
		}
		proxyURL, err = secrets.ProxyURLFromSecretRef(ctx, r.Client, proxySecretRef)
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

	if provider := repo.GetProvider(); provider != "" && provider != "generic" {
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
		var err error
		authenticator, err = authutils.GetArtifactRegistryCredentials(ctx, provider, repo.Spec.Image, opts...)
		if err != nil {
			return nil, err
		}
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

		certSecretRef := types.NamespacedName{
			Name:      certSecret.Name,
			Namespace: certSecret.Namespace,
		}
		tlsConfig, err := secrets.TLSConfigFromSecretRef(ctx, r.Client, certSecretRef)
		if err != nil {
			return nil, err
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

	if authenticator == nil {
		var pullSecrets []corev1.Secret

		if repo.Spec.SecretRef != nil {
			var s corev1.Secret
			key := types.NamespacedName{
				Name:      repo.Spec.SecretRef.Name,
				Namespace: repo.GetNamespace(),
			}
			if err := r.Get(ctx, key, &s); err != nil {
				return nil, err
			}
			pullSecrets = append(pullSecrets, s)
		}

		if repo.Spec.ServiceAccountName != "" {
			saRef := types.NamespacedName{
				Name:      repo.Spec.ServiceAccountName,
				Namespace: repo.GetNamespace(),
			}
			s, err := secrets.PullSecretsFromServiceAccountRef(ctx, r.Client, saRef)
			if err != nil {
				return nil, err
			}
			pullSecrets = append(pullSecrets, s...)
		}

		if len(pullSecrets) > 0 {
			keychain, err := k8schain.NewFromPullSecrets(ctx, pullSecrets)
			if err != nil {
				return nil, err
			}
			options = append(options, remote.WithAuthFromKeychain(keychain))
		}
	}

	return options, nil
}
