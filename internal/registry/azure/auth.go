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

package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/image-reflector-controller/internal/registry"
)

// Client is an Azure ACR client which can log into the registry and return
// authorization information.
type Client struct {
	credential azcore.TokenCredential
	scheme     string
}

// NewClient creates a new ACR client with default configurations.
func NewClient() *Client {
	return &Client{scheme: "https"}
}

// WithTokenCredential sets the token credential used by the ACR client.
func (c *Client) WithTokenCredential(tc azcore.TokenCredential) *Client {
	c.credential = tc
	return c
}

// WithScheme sets the scheme of the http request that the client makes.
func (c *Client) WithScheme(scheme string) *Client {
	c.scheme = scheme
	return c
}

// getLoginAuth returns authentication for ACR. The details needed for authentication
// are gotten from environment variable so there is not need to mount a host path.
func (c *Client) getLoginAuth(ctx context.Context, ref name.Reference) (authn.AuthConfig, error) {
	var authConfig authn.AuthConfig

	// Use default credentials if no token credential is provided.
	// NOTE: NewDefaultAzureCredential() performs a lot of environment lookup
	// for creating default token credential. Load it only when it's needed.
	if c.credential == nil {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return authConfig, err
		}
		c.credential = cred
	}

	// Obtain access token using the token credential.
	// TODO: Add support for other azure endpoints as well.
	armToken, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{string(arm.AzurePublicCloud) + ".default"},
	})
	if err != nil {
		return authConfig, err
	}

	// Obtain ACR access token using exchanger.
	endpoint := fmt.Sprintf("%s://%s", c.scheme, ref.Context().RegistryStr())
	ex := newExchanger(endpoint)
	accessToken, err := ex.ExchangeACRAccessToken(string(armToken.Token))
	if err != nil {
		return authConfig, fmt.Errorf("error exchanging token: %w", err)
	}

	return authn.AuthConfig{
		// This is the acr username used by Azure
		// See documentation: https://docs.microsoft.com/en-us/azure/container-registry/container-registry-authentication?tabs=azure-cli#az-acr-login-with---expose-token
		Username: "00000000-0000-0000-0000-000000000000",
		Password: accessToken,
	}, nil
}

// ValidHost returns if a given host is a Azure container registry.
// List from https://github.com/kubernetes/kubernetes/blob/v1.23.1/pkg/credentialprovider/azure/azure_credentials.go#L55
func ValidHost(host string) bool {
	for _, v := range []string{".azurecr.io", ".azurecr.cn", ".azurecr.de", ".azurecr.us"} {
		if strings.HasSuffix(host, v) {
			return true
		}
	}
	return false
}

// Login attempts to get the authentication material for ACR. The caller can
// ensure that the passed image is a valid ACR image using ValidHost().
func (c *Client) Login(ctx context.Context, autoLogin bool, image string, ref name.Reference) (authn.Authenticator, error) {
	if autoLogin {
		ctrl.LoggerFrom(ctx).Info("logging in to Azure ACR for " + image)
		authConfig, err := c.getLoginAuth(ctx, ref)
		if err != nil {
			ctrl.LoggerFrom(ctx).Info("error logging into ACR " + err.Error())
			return nil, err
		}

		auth := authn.FromConfig(authConfig)
		return auth, nil
	}
	ctrl.LoggerFrom(ctx).Info("ACR authentication is not enabled. To enable, set the controller flag --azure-autologin-for-acr")
	return nil, fmt.Errorf("ACR authentication failed: %w", registry.ErrUnconfiguredProvider)
}
