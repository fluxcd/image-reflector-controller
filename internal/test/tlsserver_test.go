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

package test

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
)

// Certificate code adapted from
// https://ericchiang.github.io/post/go-tls/ (license
// https://ericchiang.github.io/license/)

func TestCertAuthentication_failsWithNoClientCert(t *testing.T) {
	// This checks that _not_ using the client cert will fail;
	// i.e., that the server is expecting a valid client
	// certificate.

	g := NewWithT(t)

	srv, _, _, _, clientTLSCert, err := CreateTLSServer()
	g.Expect(err).ToNot(HaveOccurred())

	srv.StartTLS()
	defer srv.Close()

	tlsConfig := &tls.Config{}

	// Use the server cert as a CA cert, so the client trusts the
	// server cert. (Only works because the server uses the same
	// cert in both roles).
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	tlsConfig.RootCAs = pool
	// BUT: don't supply a client certificate, so the server
	// doesn't authenticate the client.
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{
		Transport: transport,
	}
	_, err = client.Get(srv.URL + "/v2/")
	g.Expect(err).ToNot(Succeed())

	// .. and this checks that using the test transport will work,
	// for the same operation.
	// Patch the client cert in as the client certificate.
	transport.TLSClientConfig.Certificates = []tls.Certificate{clientTLSCert}
	_, err = client.Get(srv.URL + "/v2/")
	g.Expect(err).To(Succeed())
}
