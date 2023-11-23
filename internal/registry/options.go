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
	"errors"
	"fmt"

	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/oci/auth/login"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/secret"
)

// AuthOptionsGetter is a function to extract information out of an ImageRepository and create
// options from it that can be used to interact with an OCI registry.
type AuthOptionsGetter func(ctx context.Context, obj imagev1.ImageRepository) ([]remote.Option, error)

// NewAuthOptionsGetter returns an AuthOptionsGetter function that builds a slice of options from an
// ImageRepository by looking up references to Secrets etc. on the Kubernetes cluster using the provided
// client interface. If no external authentication provider is configured on the ImageRepository, the given
// ProviderOptions are used for authentication. Options are extracted from the following ImageRepository spec
// fields:
//
// - spec.image
// - spec.secretRef
// - spec.provider
// - spec.certSecretRef
// - spec.serviceAccountName
func NewAuthOptionsGetter(c client.Client, deprecatedLoginOpts login.ProviderOptions) AuthOptionsGetter {
	return func(ctx context.Context, obj imagev1.ImageRepository) ([]remote.Option, error) {
		timeout := obj.GetTimeout()
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Configure authentication strategy to access the registry.
		var options []remote.Option
		var authSecret corev1.Secret
		var auth authn.Authenticator
		var authErr error

		ref, err := ParseImageReference(obj.Spec.Image)
		if err != nil {
			return nil, fmt.Errorf("failed parsing image reference %q: %w", obj.Spec.Image, err)
		}

		if obj.Spec.SecretRef != nil {
			if err := c.Get(ctx, types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      obj.Spec.SecretRef.Name,
			}, &authSecret); err != nil {
				return nil, err
			}
			auth, authErr = secret.AuthFromSecret(authSecret, ref)
		} else {
			// Build login provider options and use it to attempt registry login.
			opts := login.ProviderOptions{}
			switch obj.GetProvider() {
			case "aws":
				opts.AwsAutoLogin = true
			case "azure":
				opts.AzureAutoLogin = true
			case "gcp":
				opts.GcpAutoLogin = true
			default:
				opts = deprecatedLoginOpts
			}
			auth, authErr = login.NewManager().Login(ctx, obj.Spec.Image, ref, opts)
		}
		if authErr != nil {
			// If it's not unconfigured provider error, abort reconciliation.
			// Continue reconciliation if it's unconfigured providers for scanning
			// public repositories.
			if !errors.Is(authErr, oci.ErrUnconfiguredProvider) {
				return nil, authErr
			}
		}
		if auth != nil {
			options = append(options, remote.WithAuth(auth))
		}

		// Load any provided certificate.
		if obj.Spec.CertSecretRef != nil {
			var certSecret corev1.Secret
			if obj.Spec.SecretRef != nil && obj.Spec.SecretRef.Name == obj.Spec.CertSecretRef.Name {
				certSecret = authSecret
			} else {
				if err := c.Get(ctx, types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      obj.Spec.CertSecretRef.Name,
				}, &certSecret); err != nil {
					return nil, err
				}
			}

			tr, err := secret.TransportFromKubeTLSSecret(&certSecret)
			if err != nil {
				return nil, err
			}
			if tr.TLSClientConfig == nil {
				tr, err = secret.TransportFromSecret(&certSecret)
				if err != nil {
					return nil, err
				}
				if tr.TLSClientConfig != nil {
					ctrl.LoggerFrom(ctx).
						Info("warning: specifying TLS auth data via `certFile`/`keyFile`/`caFile` is deprecated, please use `tls.crt`/`tls.key`/`ca.crt` instead")
				}
			}
			options = append(options, remote.WithTransport(tr))
		}

		if obj.Spec.ServiceAccountName != "" {
			serviceAccount := corev1.ServiceAccount{}
			// Lookup service account
			if err := c.Get(ctx, types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      obj.Spec.ServiceAccountName,
			}, &serviceAccount); err != nil {
				return nil, err
			}

			if len(serviceAccount.ImagePullSecrets) > 0 {
				imagePullSecrets := make([]corev1.Secret, len(serviceAccount.ImagePullSecrets))
				for i, ips := range serviceAccount.ImagePullSecrets {
					var saAuthSecret corev1.Secret
					if err := c.Get(ctx, types.NamespacedName{
						Namespace: obj.GetNamespace(),
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
}
