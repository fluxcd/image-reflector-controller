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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
	"github.com/fluxcd/pkg/apis/meta"
)

// Certificate code adapted from
// https://ericchiang.github.io/post/go-tls/ (license
// https://ericchiang.github.io/license/)

var _ = Context("using TLS certificates", func() {

	var srv *httptest.Server
	var clientKey *rsa.PrivateKey
	var rootCertPEM, clientCertPEM, clientKeyPEM []byte
	var clientTLSCert tls.Certificate

	BeforeEach(func() {
		reg := &tagListHandler{
			registryHandler: registry.New(),
			imagetags:       map[string][]string{},
		}
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
		var rootCert *x509.Certificate
		rootCert, rootCertPEM, err = createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
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
		_, clientCertPEM, err = createCert(clientCertTmpl, rootCert, &clientKey.PublicKey, rootKey)
		Expect(err).To(BeNil())
		// encode and load the cert and private key for the client
		clientKeyPEM = pem.EncodeToMemory(&pem.Block{
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
		// load an image to be scanned
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
		imgRepo := loadImages(srv, "image", []string{"1.0.0"}, remote.WithTransport(transport))

		secretName := "tls-secret"
		tlsSecret := corev1.Secret{
			StringData: map[string]string{
				CACert:     string(rootCertPEM),
				ClientCert: string(clientCertPEM),
				ClientKey:  string(clientKeyPEM),
			},
		}
		tlsSecret.Name = secretName
		tlsSecret.Namespace = "default"
		Expect(k8sClient.Create(context.Background(), &tlsSecret)).To(Succeed())

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
			Name:      "scan",
			Namespace: "default",
		}
		repoObj.Name = imageRepoName.Name
		repoObj.Namespace = imageRepoName.Namespace
		Expect(k8sClient.Create(context.Background(), &repoObj)).To(Succeed())

		// wait until the controller has done something with the object
		var newImgObj imagev1.ImageRepository
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), imageRepoName, &newImgObj)
			return err == nil && len(newImgObj.Status.Conditions) > 0
		}, 10*time.Second, time.Second).Should(BeTrue())
		cond := newImgObj.Status.Conditions[0]
		Expect(cond.Type).To(Equal(meta.ReadyCondition))
		Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		// double check that it found the tag
		Expect(newImgObj.Status.LastScanResult.TagCount).To(Equal(1))
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
