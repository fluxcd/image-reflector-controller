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

package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/image-reflector-controller/internal/registry"
)

var registryPartRe = regexp.MustCompile(`([0-9+]*).dkr.ecr.([^/.]*)\.(amazonaws\.com[.cn]*)/([^:]+):?(.*)`)

// ParseImage returns the AWS account ID and region and `true` if
// the image repository is hosted in AWS's Elastic Container Registry,
// otherwise empty strings and `false`.
func ParseImage(image string) (accountId, awsEcrRegion string, ok bool) {
	registryParts := registryPartRe.FindAllStringSubmatch(image, -1)
	if len(registryParts) < 1 || len(registryParts[0]) < 3 {
		return "", "", false
	}
	return registryParts[0][1], registryParts[0][2], true
}

// Client is a AWS ECR client which can log into the registry and return
// authorization information.
type Client struct {
	*aws.Config
}

// NewClient creates a new ECR client with default configurations.
func NewClient() *Client {
	return &Client{Config: aws.NewConfig()}
}

// getLoginAuth obtains authentication for ECR given the account
// ID and region (taken from the image). This assumes that the pod has
// IAM permissions to get an authentication token, which will usually
// be the case if it's running in EKS, and may need additional setup
// otherwise (visit
// https://docs.aws.amazon.com/sdk-for-go/api/aws/session/ as a
// starting point).
func (c *Client) getLoginAuth(accountId, awsEcrRegion string) (authn.AuthConfig, error) {
	// No caching of tokens is attempted; the quota for getting an
	// auth token is high enough that getting a token every time you
	// scan an image is viable for O(500) images per region. See
	// https://docs.aws.amazon.com/general/latest/gr/ecr.html.
	var authConfig authn.AuthConfig
	accountIDs := []string{accountId}

	// Configure session.
	cfg := c.Config.WithRegion(awsEcrRegion)
	ecrService := ecr.New(session.Must(session.NewSession(cfg)))
	ecrToken, err := ecrService.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{
		RegistryIds: aws.StringSlice(accountIDs),
	})
	if err != nil {
		return authConfig, err
	}

	// Validate the authorization data.
	if len(ecrToken.AuthorizationData) == 0 {
		return authConfig, errors.New("no authorization data")
	}
	if ecrToken.AuthorizationData[0].AuthorizationToken == nil {
		return authConfig, fmt.Errorf("no authorization token")
	}
	token, err := base64.StdEncoding.DecodeString(*ecrToken.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return authConfig, err
	}

	tokenSplit := strings.Split(string(token), ":")
	// Validate the tokens.
	if len(tokenSplit) != 2 {
		// NOTE: Maybe think of some better error message?
		return authConfig, fmt.Errorf("invalid authorization token, expected to be of length 2, have %d", len(tokenSplit))
	}
	authConfig = authn.AuthConfig{
		Username: tokenSplit[0],
		Password: tokenSplit[1],
	}
	return authConfig, nil
}

// Login attempts to get the authentication material for ECR. It extracts
// the account and region information from the image URI. The caller can ensure
// that the passed image is a valid ECR image using ParseImage().
func (c *Client) Login(ctx context.Context, autoLogin bool, image string) (authn.Authenticator, error) {
	if autoLogin {
		ctrl.LoggerFrom(ctx).Info("logging in to AWS ECR for " + image)
		accountId, awsEcrRegion, ok := ParseImage(image)
		if !ok {
			return nil, errors.New("failed to parse AWS ECR image, invalid ECR image")
		}

		authConfig, err := c.getLoginAuth(accountId, awsEcrRegion)
		if err != nil {
			return nil, err
		}

		auth := authn.FromConfig(authConfig)
		return auth, nil
	}
	ctrl.LoggerFrom(ctx).Info("ECR authentication is not enabled. To enable, set the controller flag --aws-autologin-for-ecr")
	return nil, fmt.Errorf("ECR authentication failed: %w", registry.ErrUnconfiguredProvider)
}
