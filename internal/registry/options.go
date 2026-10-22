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
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	authutils "github.com/fluxcd/pkg/auth/utils"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/runtime/secrets"
	fluxbroker "github.com/fluxcd/pkg/spiffe/broker"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
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
	TokenCache            *cache.TokenCache
	DefaultServiceAccount string
	SPIFFEBroker          fluxbroker.Interface
}

func (r *AuthOptionsGetter) GetOptions(ctx context.Context, repo *imagev1.ImageRepository,
	involvedObjectKind string, involvedObject client.Object) ([]remote.Option, error) {

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

	provider := repo.GetProvider()

	// Apply the default ServiceAccount when the object does not specify one.
	saName := repo.Spec.ServiceAccountName
	if saName == "" {
		saName = r.DefaultServiceAccount
	}

	var sa *corev1.ServiceAccount
	if saName != "" {
		sa = &corev1.ServiceAccount{}
		saRef := types.NamespacedName{
			Name:      saName,
			Namespace: repo.GetNamespace(),
		}
		if err := r.Get(ctx, saRef, sa); err != nil {
			return nil, fmt.Errorf("failed to get service account '%s': %w", saRef, err)
		}
	}

	// Build the provider-agnostic authentication spec. BuildSpec is safe to call
	// even when there is no ServiceAccount or provider-specific configuration:
	// it returns a spec with a nil Auth in that case.
	buildOpts := []cryptov1.Option{
		cryptov1.WithServiceProvider(provider),
		cryptov1.WithOCI(),
	}
	if repo.Spec.SecretRef != nil {
		buildOpts = append(buildOpts, cryptov1.WithSecretRefAuth())
	}
	if sa != nil {
		buildOpts = append(buildOpts, cryptov1.WithAnnotations(sa.Annotations))
		if len(sa.ImagePullSecrets) > 0 {
			buildOpts = append(buildOpts, cryptov1.WithImagePullSecrets())
		}
	}
	if repo.Spec.CertSecretRef != nil {
		buildOpts = append(buildOpts, cryptov1.WithCertSecretRef())
	}
	spec, err := cryptov1.BuildSpec(buildOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	// Workload identity kicks in when the provider is not generic or the SA has
	// opted into object-level workload identity.
	useProviderAuth := provider != "generic" || spec.Auth.OptedIn()

	if useProviderAuth {
		authOpts := []auth.Option{
			auth.WithClient(r.Client),
			auth.WithObject(imagev1.GroupVersion.Group, imagev1.ImageRepositoryKind, repo),
		}
		authOpts = append(authOpts, auth.OptionsFromSpec(spec.Auth)...)
		if r.SPIFFEBroker != nil {
			authOpts = append(authOpts, auth.WithSPIFFEBroker(r.SPIFFEBroker))
		}
		if sa != nil {
			authOpts = append(authOpts, auth.WithServiceAccountName(sa.Name))
		}
		if proxyURL != nil {
			authOpts = append(authOpts, auth.WithProxyURL(*proxyURL))
		}
		if r.TokenCache != nil {
			authOpts = append(authOpts,
				auth.WithCache(*r.TokenCache),
				auth.WithCacheOperation(cache.OperationReconcile),
				auth.WithInvolvedObject(imagev1.GroupVersion.Group, involvedObjectKind, involvedObject))
		}
		authenticator, err = authutils.GetArtifactRegistryCredentials(ctx, provider, repo.Spec.Image, authOpts...)
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

		// NOTE: Use WithSystemCertPool to maintain backward compatibility with the existing
		// extend approach (system CAs + user CA) rather than the default replace approach (user CA only).
		// This ensures image-reflector-controller continues to work with both system and user-provided CA certificates.
		var tlsOpts = []secrets.TLSConfigOption{secrets.WithSystemCertPool()}
		tlsConfig, err := secrets.TLSConfigFromSecretRef(ctx, r.Client, certSecretRef, tlsOpts...)
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

		if sa != nil {
			s, err := secrets.PullSecretsFromServiceAccount(ctx, r.Client, sa)
			if err != nil {
				return nil, err
			}
			pullSecrets = append(pullSecrets, s...)
		}

		if len(pullSecrets) > 0 {
			keychain, err := kauth.NewFromPullSecrets(ctx, pullSecrets)
			if err != nil {
				return nil, err
			}
			options = append(options, remote.WithAuthFromKeychain(keychain))
		}
	}

	return options, nil
}
