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

package controller

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/secret"
	"github.com/fluxcd/image-reflector-controller/internal/test"
)

// mockDatabase mocks the image repository database.
type mockDatabase struct {
	TagData    []string
	ReadError  error
	WriteError error
}

// SetTags implements the DatabaseWriter interface of the Database.
func (db *mockDatabase) SetTags(repo string, tags []string) error {
	if db.WriteError != nil {
		return db.WriteError
	}
	db.TagData = append(db.TagData, tags...)
	return nil
}

// Tags implements the DatabaseReader interface of the Database.
func (db mockDatabase) Tags(repo string) ([]string, error) {
	if db.ReadError != nil {
		return nil, db.ReadError
	}
	return db.TagData, nil
}

func TestImageRepositoryReconciler_deleteBeforeFinalizer(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagerepo-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imagerepo := &imagev1.ImageRepository{}
	imagerepo.Name = "test-gitrepo"
	imagerepo.Namespace = namespaceName
	imagerepo.Spec = imagev1.ImageRepositorySpec{
		Interval: metav1.Duration{Duration: interval},
		Image:    "test-image",
	}
	// Add a test finalizer to prevent the object from getting deleted.
	imagerepo.SetFinalizers([]string{"test-finalizer"})
	g.Expect(k8sClient.Create(ctx, imagerepo)).NotTo(HaveOccurred())
	// Add deletion timestamp by deleting the object.
	g.Expect(k8sClient.Delete(ctx, imagerepo)).NotTo(HaveOccurred())

	r := &ImageRepositoryReconciler{
		Client:        k8sClient,
		EventRecorder: record.NewFakeRecorder(32),
	}
	// NOTE: Only a real API server responds with an error in this scenario.
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(imagerepo)})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestImageRepositoryReconciler_setAuthOptions(t *testing.T) {
	testImg := "example.com/foo/bar"
	testSecretName := "test-secret"
	testTLSSecretName := "test-tls-secret"
	testDeprecatedTLSSecretName := "test-deprecated-tls-secret"
	testServiceAccountName := "test-service-account"
	testNamespace := "test-ns"

	dockerconfigjson := []byte(`
{
	"auths": {
		"example.com": {
			"username": "user",
			"password": "pass"
		}
	}
}`)

	testSecret := &corev1.Secret{}
	testSecret.Name = testSecretName
	testSecret.Namespace = testNamespace
	testSecret.Type = corev1.SecretTypeDockerConfigJson
	testSecret.Data = map[string][]byte{".dockerconfigjson": dockerconfigjson}
	g := NewWithT(t)

	// Create a test TLS server to get valid cert data. The server is never
	// started or used below.
	_, rootCertPEM, clientCertPEM, clientKeyPEM, _, err := test.CreateTLSServer()
	g.Expect(err).To(Not(HaveOccurred()))

	testTLSSecret := &corev1.Secret{}
	testTLSSecret.Name = testTLSSecretName
	testTLSSecret.Namespace = testNamespace
	testTLSSecret.Type = corev1.SecretTypeTLS
	testTLSSecret.Data = map[string][]byte{
		secret.CACrtKey:         rootCertPEM,
		corev1.TLSCertKey:       clientCertPEM,
		corev1.TLSPrivateKeyKey: clientKeyPEM,
	}

	testDeprecatedTLSSecret := &corev1.Secret{}
	testDeprecatedTLSSecret.Name = testDeprecatedTLSSecretName
	testDeprecatedTLSSecret.Namespace = testNamespace
	testDeprecatedTLSSecret.Type = corev1.SecretTypeTLS
	testDeprecatedTLSSecret.Data = map[string][]byte{
		secret.CACert:     rootCertPEM,
		secret.ClientCert: clientCertPEM,
		secret.ClientKey:  clientKeyPEM,
	}

	// Docker config secret with TLS data.
	testDockerCfgSecretWithTLS := testSecret.DeepCopy()
	testDockerCfgSecretWithTLS.Data = map[string][]byte{
		secret.CACrtKey:         rootCertPEM,
		corev1.TLSCertKey:       clientCertPEM,
		corev1.TLSPrivateKeyKey: clientKeyPEM,
	}

	// ServiceAccount without image pull secret.
	testServiceAccount := &corev1.ServiceAccount{}
	testServiceAccount.Name = testServiceAccountName
	testServiceAccount.Namespace = testNamespace

	// ServiceAccount with image pull secret.
	testServiceAccountWithSecret := testServiceAccount.DeepCopy()
	testServiceAccountWithSecret.ImagePullSecrets = []corev1.LocalObjectReference{{Name: testSecretName}}

	tests := []struct {
		name          string
		mockObjs      []client.Object
		imageRepoSpec imagev1.ImageRepositorySpec
		wantErr       bool
	}{
		{
			name: "no auth options",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
			},
		},
		{
			name:     "secret ref with existing secret",
			mockObjs: []client.Object{testSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				SecretRef: &meta.LocalObjectReference{
					Name: testSecretName,
				},
			},
		},
		{
			name: "secret ref with non-existing secret",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				SecretRef: &meta.LocalObjectReference{
					Name: "non-existing-secret",
				},
			},
			wantErr: true,
		},
		{
			name: "contextual login",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:    "123456789000.dkr.ecr.us-east-2.amazonaws.com/test",
				Provider: "aws",
			},
			wantErr: true,
		},
		{
			name: "cloud provider repo without login",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: "123456789000.dkr.ecr.us-east-2.amazonaws.com/test",
			},
		},
		{
			name:     "cert secret ref with existing secret",
			mockObjs: []client.Object{testTLSSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: testTLSSecretName,
				},
			},
		},
		{
			name:     "cert secret ref with existing secret using deprecated keys",
			mockObjs: []client.Object{testDeprecatedTLSSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: testDeprecatedTLSSecretName,
				},
			},
		},
		{
			name: "cert secret ref with non-existing secret",
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: "non-existing-secret",
				},
			},
			wantErr: true,
		},
		{
			name:     "secret ref and cert secret ref",
			mockObjs: []client.Object{testSecret, testTLSSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				SecretRef: &meta.LocalObjectReference{
					Name: testSecretName,
				},
				CertSecretRef: &meta.LocalObjectReference{
					Name: testTLSSecretName,
				},
			},
		},
		{
			name:     "cert secret ref of type docker config",
			mockObjs: []client.Object{testDockerCfgSecretWithTLS},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image: testImg,
				CertSecretRef: &meta.LocalObjectReference{
					Name: testSecretName,
				},
			},
			wantErr: true,
		},
		{
			name:     "service account without pull secret",
			mockObjs: []client.Object{testServiceAccount},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:              testImg,
				ServiceAccountName: testServiceAccountName,
			},
		},
		{
			name:     "service account with pull secret",
			mockObjs: []client.Object{testServiceAccountWithSecret, testSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:              testImg,
				ServiceAccountName: testServiceAccountName,
			},
		},
		{
			name:     "service account with non-existing pull secret",
			mockObjs: []client.Object{testServiceAccountWithSecret},
			imageRepoSpec: imagev1.ImageRepositorySpec{
				Image:              testImg,
				ServiceAccountName: testServiceAccountName,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			clientBuilder := fake.NewClientBuilder()
			clientBuilder.WithObjects(tt.mockObjs...)

			r := &ImageRepositoryReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Client:        clientBuilder.Build(),
				patchOptions:  getPatchOptions(imageRepositoryOwnedConditions, "irc"),
			}

			obj := &imagev1.ImageRepository{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "reconcile-repo-",
					Generation:   1,
					Namespace:    testNamespace,
				},
			}
			obj.Spec = tt.imageRepoSpec

			ref, err := name.ParseReference(obj.Spec.Image)
			g.Expect(err).ToNot(HaveOccurred())

			_, err = r.setAuthOptions(ctx, obj, ref)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func TestImageRepositoryReconciler_shouldScan(t *testing.T) {
	testImage := "example.com/foo/bar"
	tests := []struct {
		name          string
		beforeFunc    func(obj *imagev1.ImageRepository, reconcileTime time.Time)
		db            *mockDatabase
		reconcileTime time.Time
		wantErr       bool
		wantScan      bool
		wantNextScan  time.Duration
		wantReason    string
	}{
		{
			name:         "new object",
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonNeverScanned,
		},
		{
			name: "first reconcile at annotation",
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: "now"})
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.Time{Time: reconcileTime.Add(-time.Second * 30)},
				}
			},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonReconcileRequested,
		},
		{
			name: "second reconcile at annotation",
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: "now"})
				obj.Status.LastHandledReconcileAt = "foo"
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.Time{Time: reconcileTime.Add(-time.Second * 30)},
				}
			},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonReconcileRequested,
		},
		{
			name:          "reconcile at annotation with same value",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: "now"})
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastHandledReconcileAt = "now"
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     false,
			wantNextScan: time.Second * 30,
		},
		{
			name:          "change image",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				newImage := "example.com/other/image"
				obj.Spec.Image = newImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonNewImageName,
		},
		{
			name:          "exclusion list change",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.ObservedExclusionList = []string{"baz"}
				obj.Spec.ExclusionList = []string{"bar"}
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonUpdatedExclusionList,
		},
		{
			name:          "no tags",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					TagCount: 0,
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 10)),
				}
			},
			db:           &mockDatabase{},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonEmptyDatabase,
		},
		{
			name:          "database read failure",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Second * 30)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}, ReadError: errors.New("fail")},
			wantErr:      true,
			wantScan:     false,
			wantNextScan: time.Minute,
		},
		{
			name:          "after the interval",
			reconcileTime: time.Now(),
			beforeFunc: func(obj *imagev1.ImageRepository, reconcileTime time.Time) {
				obj.Status.CanonicalImageName = testImage
				obj.Status.LastScanResult = &imagev1.ScanResult{
					ScanTime: metav1.NewTime(reconcileTime.Add(-time.Minute * 2)),
				}
			},
			db:           &mockDatabase{TagData: []string{"foo"}},
			wantScan:     true,
			wantNextScan: time.Minute,
			wantReason:   scanReasonInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			r := &ImageRepositoryReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Database:      tt.db,
				patchOptions:  getPatchOptions(imageRepositoryOwnedConditions, "irc"),
			}

			obj := &imagev1.ImageRepository{}
			obj.Spec.Image = testImage
			obj.Spec.Interval = metav1.Duration{Duration: time.Minute}
			obj.Spec.ExclusionList = []string{"aaa"}
			obj.Status.ObservedExclusionList = []string{"aaa"}

			if tt.beforeFunc != nil {
				tt.beforeFunc(obj, tt.reconcileTime)
			}

			scan, next, scanReason, err := r.shouldScan(*obj, tt.reconcileTime)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			g.Expect(scan).To(Equal(tt.wantScan))
			g.Expect(next).To(Equal(tt.wantNextScan))
			g.Expect(scanReason).To(Equal(tt.wantReason))
		})
	}
}

func TestImageRepositoryReconciler_scan(t *testing.T) {
	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	tests := []struct {
		name           string
		tags           []string
		exclusionList  []string
		annotation     string
		db             *mockDatabase
		wantErr        bool
		wantTags       []string
		wantLatestTags []string
	}{
		{
			name:    "no tags",
			wantErr: true,
		},
		{
			name:           "simple tags",
			tags:           []string{"a", "b", "c", "d"},
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b", "c", "d"},
			wantLatestTags: []string{"d", "c", "b", "a"},
		},
		{
			name:           "simple tags, 10+",
			tags:           []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			wantLatestTags: []string{"k", "j", "i", "h, g, f, e, d, c, b"},
		},
		{
			name:           "with single exclusion pattern",
			tags:           []string{"a", "b", "c", "d"},
			exclusionList:  []string{"c"},
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b", "d"},
			wantLatestTags: []string{"d", "b", "a"},
		},
		{
			name:           "with multiple exclusion pattern",
			tags:           []string{"a", "b", "c", "d"},
			exclusionList:  []string{"c", "a"},
			db:             &mockDatabase{},
			wantTags:       []string{"b", "d"},
			wantLatestTags: []string{"d", "b"},
		},
		{
			name:          "bad exclusion pattern",
			tags:          []string{"a"}, // Ensure repo isn't empty to prevent 404.
			exclusionList: []string{"[="},
			wantErr:       true,
		},
		{
			name:    "db write fails",
			tags:    []string{"a", "b"},
			db:      &mockDatabase{WriteError: errors.New("fail")},
			wantErr: true,
		},
		{
			name:           "with reconcile annotation",
			tags:           []string{"a", "b"},
			annotation:     "foo",
			db:             &mockDatabase{},
			wantTags:       []string{"a", "b"},
			wantLatestTags: []string{"b", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			imgRepo, err := test.LoadImages(registryServer, "test-fetch-"+randStringRunes(5), tt.tags)
			g.Expect(err).ToNot(HaveOccurred())

			r := ImageRepositoryReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Database:      tt.db,
				patchOptions:  getPatchOptions(imageRepositoryOwnedConditions, "irc"),
			}

			repo := &imagev1.ImageRepository{}
			repo.Spec = imagev1.ImageRepositorySpec{
				Image:         imgRepo,
				ExclusionList: tt.exclusionList,
			}

			if tt.annotation != "" {
				repo.SetAnnotations(map[string]string{meta.ReconcileRequestAnnotation: tt.annotation})
			}

			ref, err := parseImageReference(imgRepo, false)
			g.Expect(err).ToNot(HaveOccurred())

			opts := []remote.Option{}

			tagCount, err := r.scan(context.TODO(), repo, ref, opts)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if err == nil {
				g.Expect(tagCount).To(Equal(len(tt.wantTags)))
				g.Expect(r.Database.Tags(imgRepo)).To(Equal(tt.wantTags))
				g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(tt.wantTags)))
				g.Expect(repo.Status.LastScanResult.ScanTime).ToNot(BeZero())
				if tt.annotation != "" {
					g.Expect(repo.Status.LastHandledReconcileAt).To(Equal(tt.annotation))
				}
			}
		})
	}
}

func TestGetLatestTags(t *testing.T) {
	tests := []struct {
		name           string
		tags           []string
		wantLatestTags []string
	}{
		{
			name:           "no tags",
			wantLatestTags: nil,
		},
		{
			name:           "few semver tags",
			tags:           []string{"1.0.0", "0.0.8", "1.2.5", "3.0.1", "1.0.1"},
			wantLatestTags: []string{"3.0.1", "1.2.5", "1.0.1", "1.0.0", "0.0.8"},
		},
		{
			name:           "10 semver tags",
			tags:           []string{"1.0.0", "0.0.8", "1.2.5", "3.0.1", "1.0.1", "5.1.1", "4.1.0", "4.5.0", "4.0.3", "2.2.2"},
			wantLatestTags: []string{"5.1.1", "4.5.0", "4.1.0", "4.0.3", "3.0.1", "2.2.2", "1.2.5", "1.0.1", "1.0.0", "0.0.8"},
		},
		{
			name:           "10+ semver tags",
			tags:           []string{"1.0.0", "0.0.8", "1.2.5", "3.0.1", "1.0.1", "5.1.1", "4.1.0", "4.5.0", "4.0.3", "2.2.2", "0.5.1", "0.1.0"},
			wantLatestTags: []string{"5.1.1", "4.5.0", "4.1.0", "4.0.3", "3.0.1", "2.2.2", "1.2.5", "1.0.1", "1.0.0", "0.5.1"},
		},
		{
			name:           "few numerical tags",
			tags:           []string{"-62", "-88", "73", "72", "15"},
			wantLatestTags: []string{"73", "72", "15", "-88", "-62"},
		},
		{
			name:           "few numerical tags",
			tags:           []string{"-62", "-88", "73", "72", "15", "16", "15", "29", "-33", "-91", "100", "101"},
			wantLatestTags: []string{"73", "72", "29", "16", "15", "15", "101", "100", "-91", "-88"},
		},
		{
			name:           "few word tags",
			tags:           []string{"aaa", "bbb", "ccc"},
			wantLatestTags: []string{"ccc", "bbb", "aaa"},
		},
		{
			name:           "few word tags",
			tags:           []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff", "ggg", "hhh", "iii", "jjj", "kkk", "lll"},
			wantLatestTags: []string{"lll", "kkk", "jjj", "iii", "hhh", "ggg", "fff", "eee", "ddd", "ccc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(getLatestTags(tt.tags)).To(Equal(tt.wantLatestTags))
		})
	}
}

func Test_parseImageReference(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		insecure bool
		wantErr  bool
		wantRef  string
	}{
		{
			name:    "simple valid url",
			url:     "example.com/foo/bar",
			wantRef: "example.com/foo/bar",
		},
		{
			name:    "with scheme prefix",
			url:     "https://example.com/foo/bar",
			wantErr: true,
		},
		{
			name:    "with tag",
			url:     "example.com/foo/bar:baz",
			wantErr: true,
		},
		{
			name:    "with host port",
			url:     "example.com:9999/foo/bar",
			wantErr: false,
			wantRef: "example.com:9999/foo/bar",
		},
		{
			name:     "with insecure registry",
			url:      "example.com/foo/bar",
			insecure: true,
			wantRef:  "example.com/foo/bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			ref, err := parseImageReference(tt.url, tt.insecure)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(ref.String()).To(Equal(tt.wantRef))
				if tt.insecure {
					g.Expect(ref.Context().Registry.Scheme()).To(Equal("http"))
				}
			}
		})
	}
}

func TestFilterOutTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		patterns []string
		wantErr  bool
		wantTags []string
	}{
		{
			name:     "no pattern",
			tags:     []string{"a", "b", "c", "d"},
			wantTags: []string{"a", "b", "c", "d"},
		},
		{
			name:     "single patterns",
			tags:     []string{"a", "b", "c", "d"},
			patterns: []string{"[abc]"},
			wantTags: []string{"d"},
		},
		{
			name:     "multiple patterns",
			tags:     []string{"a", "b", "c", "d"},
			patterns: []string{"[a]", "[d]"},
			wantTags: []string{"b", "c"},
		},
		{
			name:     "invalid pattern",
			patterns: []string{"[="},
			wantErr:  true,
		},
		{
			name:     "version tags",
			tags:     []string{"0.1.0", "0.2.0", "0.2.-alpha", "0.3.0", "0.4.0", "0.4.0.sig"},
			patterns: []string{"^.*\\-alpha$", "^.*\\.sig$"},
			wantTags: []string{"0.1.0", "0.2.0", "0.3.0", "0.4.0"},
		},
		{
			name:     "multiple matches in single pattern",
			tags:     []string{"aaa", "bbb", "ccc", "ddd"},
			patterns: []string{"aaa|ccc"},
			wantTags: []string{"bbb", "ddd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := filterOutTags(tt.tags, tt.patterns)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			g.Expect(result).To(Equal(tt.wantTags))
		})
	}
}

func TestIsEqualSliceContent(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "empty equal",
			want: true,
		},
		{
			name: "equal",
			a:    []string{"foo1", "bar1"},
			b:    []string{"foo1", "bar1"},
			want: true,
		},
		{
			name: "same length, different content",
			a:    []string{"foo1", "bar1"},
			b:    []string{"foo2", "bar1"},
			want: false,
		},
		{
			name: "different content length",
			a:    []string{"foo1", "bar1"},
			b:    []string{"foo1", "bar1", "baz1"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(isEqualSliceContent(tt.a, tt.b)).To(Equal(tt.want))
		})
	}
}

func TestNotify(t *testing.T) {
	nextScanMsg := "foo"
	tests := []struct {
		name       string
		beforeFunc func(oldObj, newObj *imagev1.ImageRepository)
		wantEvent  string
	}{
		{
			name: "first time success reconcile, empty old object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
			},
			wantEvent: "Normal Succeeded found x tags",
		},
		{
			name: "no-op reconcile, same old and new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
			},
			wantEvent: "Trace Succeeded foo",
		},
		{
			name: "new tags, ready but different old and new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found y tags")
			},
			wantEvent: "Normal Succeeded found y tags",
		},
		{
			name: "ready old object, not ready new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkFalse(newObj, meta.ReadyCondition, meta.FailedReason, "scan failed")
			},
			wantEvent: "Warning Failed scan failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			recorder := record.NewFakeRecorder(32)

			oldObj := &imagev1.ImageRepository{}
			newObj := oldObj.DeepCopy()

			if tt.beforeFunc != nil {
				tt.beforeFunc(oldObj, newObj)
			}

			notify(context.TODO(), recorder, oldObj, newObj, nextScanMsg)

			select {
			case x, ok := <-recorder.Events:
				g.Expect(ok).To(Equal(tt.wantEvent != ""), "unexpected event received")
				if tt.wantEvent != "" {
					g.Expect(x).To(ContainSubstring(tt.wantEvent))
				}
			default:
				if tt.wantEvent != "" {
					t.Errorf("expected some event to be emitted")
				}
			}
		})
	}
}

func TestImageRepositoryReconciler_TLS(t *testing.T) {
	g := NewWithT(t)

	// Run test registry server.
	srv, rootCertPEM, clientCertPEM, clientKeyPEM, clientTLSCert, err := test.CreateTLSServer()
	g.Expect(err).To(Not(HaveOccurred()))
	srv.StartTLS()
	defer srv.Close()

	// Construct a test repository reference.
	u, err := url.Parse(srv.URL)
	g.Expect(err).ToNot(HaveOccurred())
	repoURL := fmt.Sprintf("%s/foo", u.Host)
	ref, err := name.ParseReference(repoURL)
	g.Expect(err).ToNot(HaveOccurred())

	// Push a test image.
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	remoteOpts := []remote.Option{remote.WithTransport(&http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:      pool,
			Certificates: []tls.Certificate{clientTLSCert},
		},
	})}
	img, err := random.Image(1024, 1)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(remote.Write(ref, img, remoteOpts...)).ToNot(HaveOccurred())
	dst := ref.Context().Tag("v1.2.3")
	desc, err := remote.Get(ref, remoteOpts...)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(remote.Tag(dst, desc, remoteOpts...))

	// Construct cert secret.
	testSecretName := "test-secret"
	testNamespace := "test-ns" + randStringRunes(5)

	testTLSSecret := &corev1.Secret{}
	testTLSSecret.Name = testSecretName
	testTLSSecret.Namespace = testNamespace
	testTLSSecret.Type = corev1.SecretTypeTLS
	testTLSSecret.Data = map[string][]byte{
		secret.CACrtKey:         rootCertPEM,
		corev1.TLSCertKey:       clientCertPEM,
		corev1.TLSPrivateKeyKey: clientKeyPEM,
	}

	// Construct ImageRepository.
	obj := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo-" + randStringRunes(5),
			Namespace: testNamespace,
		},
		Spec: imagev1.ImageRepositorySpec{
			Image: repoURL,
			CertSecretRef: &meta.LocalObjectReference{
				Name: testSecretName,
			},
		},
	}

	clientBuilder := fake.NewClientBuilder().
		WithScheme(testEnv.GetScheme()).
		WithObjects(testTLSSecret, obj).
		WithStatusSubresource(&imagev1.ImageRepository{})

	r := &ImageRepositoryReconciler{
		EventRecorder: record.NewFakeRecorder(32),
		Client:        clientBuilder.Build(),
		patchOptions:  getPatchOptions(imageRepositoryOwnedConditions, "irc"),
		Database:      &mockDatabase{},
	}

	sp := patch.NewSerialPatcher(obj, r.Client)
	_, err = r.reconcile(ctx, sp, obj, time.Now())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.IsReady(obj)).To(BeTrue())
}
