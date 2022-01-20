/*
MIT License

Copyright (c) Microsoft Corporation.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE
*/

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
This package exchanges an ARM access token for an ACR access token on Azure
It has been derived from
https://github.com/Azure/msi-acrpull/blob/main/pkg/authorizer/token_exchanger.go
since the project isn't actively maintained.
*/

package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Resource     string `json:"resource"`
	TokenType    string `json:"token_type"`
}

type acrError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Exchanger struct {
	acrFQDN string
}

func NewExchanger(acrEndpoint string) *Exchanger {
	return &Exchanger{
		acrFQDN: acrEndpoint,
	}
}

func (e *Exchanger) ExchangeACRAccessToken(armToken string) (string, error) {
	exchangeUrl := fmt.Sprintf("https://%s/oauth2/exchange", e.acrFQDN)
	parsedURL, err := url.Parse(exchangeUrl)
	if err != nil {
		return "", err
	}

	parameters := url.Values{}
	parameters.Add("grant_type", "access_token")
	parameters.Add("service", parsedURL.Hostname())
	parameters.Add("access_token", armToken)

	resp, err := http.PostForm(exchangeUrl, parameters)
	if err != nil {
		return "", fmt.Errorf("failed to send token exchange request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errors []acrError
		decoder := json.NewDecoder(resp.Body)
		if err = decoder.Decode(&errors); err == nil {
			return "", fmt.Errorf("unexpected status code %d from exchnage request: errors:%s",
				resp.StatusCode, errors)
		}

		return "", fmt.Errorf("unexpected status code %d from exchnage request", resp.StatusCode)
	}

	var tokenResp tokenResponse
	decoder := json.NewDecoder(resp.Body)
	if err = decoder.Decode(&tokenResp); err != nil {
		return "", err
	}
	return tokenResp.RefreshToken, nil
}
