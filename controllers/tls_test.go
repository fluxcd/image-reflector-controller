/*
Copyright 2020, 2021 The Flux authors

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

package controllers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"math/big"
	"net"
	"time"
	//	"fmt"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	//	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	//	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Certificate code adapted from
// https://ericchiang.github.io/post/go-tls/ (license
// https://ericchiang.github.io/license/)

var _ = Context("using TLS certificates", func() {

	var srv *httptest.Server
	var clientKey *rsa.PrivateKey
	var clientTLSCert tls.Certificate

	BeforeEach(func() {
		reg := registry.New()
		srv = httptest.NewUnstartedServer(reg)

		// create a self-signed cert to use as the CA and server cert.
		rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).To(BeNil())
		rootCertTmpl, err := certTemplate()
		Expect(err).To(BeNil())
		rootCertTmpl.IsCA = true
		rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
		rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		rootCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
		rootCert, rootCertPEM, err := createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
		Expect(err).To(BeNil())

		rootKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
		})

		// Create a TLS cert using the private key and certificate
		rootTLSCert, err := tls.X509KeyPair(rootCertPEM, rootKeyPEM)
		Expect(err).To(BeNil())

		// To trust a client certificate, the server must be given a
		// CA cert pool.
		pool := x509.NewCertPool()
		pool.AddCert(rootCert)

		srv.TLS = &tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{rootTLSCert},
			ClientCAs:    pool,
		}
		// StartTLS will use the certificate given as the server cert.
		srv.StartTLS()

		// create a client cert, signed by the "CA"
		clientKey, err = rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).To(BeNil())
		clientCertTmpl, err := certTemplate()
		Expect(err).To(BeNil())
		clientCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
		clientCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		_, clientCertPEM, err := createCert(clientCertTmpl, rootCert, &clientKey.PublicKey, rootKey)
		Expect(err).To(BeNil())
		// encode and load the cert and private key for the client
		clientKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
		})
		clientTLSCert, err = tls.X509KeyPair(clientCertPEM, clientKeyPEM)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		srv.Close()
	})

	It("fails to authenticate with no client cert", func() {
		// This checks that _not_ using the client cert will fail;
		// i.e., that the server is expecting a valid client
		// certificate.
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
		_, err := client.Get(srv.URL + "/v2/")
		Expect(err).ToNot(Succeed())

		// .. and this checks that using the test transport will work,
		// for the same operation.
		// Patch the client cert in as the client certificate
		transport.TLSClientConfig.Certificates = []tls.Certificate{clientTLSCert}
		_, err = client.Get(srv.URL + "/v2/")
		Expect(err).To(Succeed())
	})

	It("can scan using certs from the secret", func() {
	})
})

// These two taken verbatim from https://ericchiang.github.io/post/go-tls/

func certTemplate() (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would
	// have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"Flux project"}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour), // valid for an hour
		BasicConstraintsValid: true,
	}
	return &tmpl, nil
}

func createCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (
	cert *x509.Certificate, certPEM []byte, err error) {

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}

// ----
