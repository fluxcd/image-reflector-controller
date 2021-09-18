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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
)

// Certificate code adapted from
// https://ericchiang.github.io/post/go-tls/ (license
// https://ericchiang.github.io/license/)

func TestCertAuthentication_failsWithNoClientCert(t *testing.T) {
	// This checks that _not_ using the client cert will fail;
	// i.e., that the server is expecting a valid client
	// certificate.

	g := NewWithT(t)

	srv, _, _, _, clientTLSCert, err := createTLSServer()
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

func TestCertAuthentication_scanWithCertsFromSecret(t *testing.T) {
	g := NewWithT(t)

	srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err := createTLSServer()
	g.Expect(err).ToNot(HaveOccurred())

	srv.StartTLS()
	defer srv.Close()

	// Load an image to be scanned.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	// Use the server cert as a CA cert, so the client trusts the
	// server cert. (Only works because the server uses the same
	// cert in both roles).
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	transport.TLSClientConfig.RootCAs = pool
	transport.TLSClientConfig.Certificates = []tls.Certificate{clientTLSCert}
	imgRepo, err := loadImages(srv, "image-"+randStringRunes(5), []string{"1.0.0"}, remote.WithTransport(transport))
	g.Expect(err).ToNot(HaveOccurred())

	secretName := "tls-secret-" + randStringRunes(5)
	tlsSecret := corev1.Secret{
		StringData: map[string]string{
			CACert:     string(rootCertPEM),
			ClientCert: string(clientCertPEM),
			ClientKey:  string(clientKeyPEM),
		},
	}
	tlsSecret.Name = secretName
	tlsSecret.Namespace = "default"

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	g.Expect(testEnv.Create(ctx, &tlsSecret)).To(Succeed())

	repoObj := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: time.Hour},
			Image:    imgRepo,
			CertSecretRef: &meta.LocalObjectReference{
				Name: secretName,
			},
		},
	}
	imageRepoName := types.NamespacedName{
		Name:      "scan-" + randStringRunes(5),
		Namespace: "default",
	}
	repoObj.Name = imageRepoName.Name
	repoObj.Namespace = imageRepoName.Namespace
	g.Expect(testEnv.Create(ctx, &repoObj)).To(Succeed())

	// Wait until the controller has done something with the object.
	var newImgObj imagev1.ImageRepository
	g.Eventually(func() bool {
		err := testEnv.Get(ctx, imageRepoName, &newImgObj)
		return err == nil && len(newImgObj.Status.Conditions) > 0
	}, 10*time.Second, time.Second).Should(BeTrue())
	cond := newImgObj.Status.Conditions[0]
	g.Expect(cond.Type).To(Equal(meta.ReadyCondition))
	g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	// Double check that it found the tag.
	g.Expect(newImgObj.Status.LastScanResult.TagCount).To(Equal(1))
}

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

func createTLSServer() (*httptest.Server, []byte, []byte, []byte, tls.Certificate, error) {
	var clientTLSCert tls.Certificate
	var rootCertPEM, clientCertPEM, clientKeyPEM []byte

	reg := &tagListHandler{
		registryHandler: registry.New(),
		imagetags:       map[string][]string{},
	}
	srv := httptest.NewUnstartedServer(reg)

	// Create a self-signed cert to use as the CA and server cert.
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
	}
	rootCertTmpl, err := certTemplate()
	if err != nil {
		return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
	}
	rootCertTmpl.IsCA = true
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	rootCertTmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	var rootCert *x509.Certificate
	rootCert, rootCertPEM, err = createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
	}

	rootKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
	})

	// Create a TLS cert using the private key and certificate.
	rootTLSCert, err := tls.X509KeyPair(rootCertPEM, rootKeyPEM)
	if err != nil {
		return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
	}

	// To trust a client certificate, the server must be given a
	// CA cert pool.
	pool := x509.NewCertPool()
	pool.AddCert(rootCert)

	srv.TLS = &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{rootTLSCert},
		ClientCAs:    pool,
	}

	// Create a client cert, signed by the "CA".
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
	}
	clientCertTmpl, err := certTemplate()
	if err != nil {
		return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
	}
	clientCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	clientCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	_, clientCertPEM, err = createCert(clientCertTmpl, rootCert, &clientKey.PublicKey, rootKey)
	if err != nil {
		return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
	}
	// Encode and load the cert and private key for the client.
	clientKeyPEM = pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	})
	clientTLSCert, err = tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	return srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err
}
