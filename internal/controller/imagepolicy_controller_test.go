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
	"errors"
	"testing"

	aclapis "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/oci/auth/login"
	"github.com/fluxcd/pkg/runtime/acl"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/policy"
	"github.com/fluxcd/image-reflector-controller/internal/registry"
	"github.com/fluxcd/image-reflector-controller/internal/test"
)

func TestImagePolicyReconciler_deleteBeforeFinalizer(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imagePolicy := &imagev1.ImagePolicy{}
	imagePolicy.Name = "test-imagepolicy"
	imagePolicy.Namespace = namespaceName
	imagePolicy.Spec = imagev1.ImagePolicySpec{
		ImageRepositoryRef: meta.NamespacedObjectReference{
			Name: "foo",
		},
		Policy: imagev1.ImagePolicyChoice{},
	}
	// Add a test finalizer to prevent the object from getting deleted.
	imagePolicy.SetFinalizers([]string{"test-finalizer"})
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
	// Add deletion timestamp by deleting the object.
	g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())

	r := &ImagePolicyReconciler{
		Client:        k8sClient,
		EventRecorder: record.NewFakeRecorder(32),
	}
	// NOTE: Only a real API server responds with an error in this scenario.
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(imagePolicy)})
	g.Expect(err).NotTo(HaveOccurred())
}

func TestStatusMigrationToImageRef(t *testing.T) {
	g := NewWithT(t)

	s := runtime.NewScheme()
	utilruntime.Must(imagev1.AddToScheme(s))
	utilruntime.Must(corev1.AddToScheme(s))

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "imagepolicy-" + randStringRunes(5),
		},
	}

	imageRepo := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.Name,
			Name:      "status-migration-test",
		},
		Spec: imagev1.ImageRepositorySpec{
			Image: "ghcr.io/stefanprodan/podinfo",
		},
		Status: imagev1.ImageRepositoryStatus{
			LastScanResult: &imagev1.ScanResult{
				TagCount:   3,
				LatestTags: []string{"1.0.0", "1.1.0", "2.0.0"},
			},
		},
	}
	imagePol := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  ns.Name,
			Name:       "status-migration-test",
			Generation: 1,
			Finalizers: []string{imagev1.ImageFinalizer},
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				SemVer: &imagev1.SemVerPolicy{
					Range: "1.0",
				},
			},
		},
		Status: imagev1.ImagePolicyStatus{
			LatestImage: "ghcr.io/stefanprodan/podinfo:1.0.0",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(ns, imageRepo, imagePol).
		WithStatusSubresource(imagePol).
		Build()

	r := &ImagePolicyReconciler{
		EventRecorder: record.NewFakeRecorder(32),
		Client:        c,
		Database:      &mockDatabase{TagData: imageRepo.Status.LastScanResult.LatestTags},
		AuthOptionsGetter: registry.NewAuthOptionsGetter(c, login.ProviderOptions{
			AwsAutoLogin:   false,
			AzureAutoLogin: false,
			GcpAutoLogin:   false,
		}),
	}
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: ns.Name,
			Name:      imagePol.Name,
		},
	})

	g.Expect(err).NotTo(HaveOccurred(), "reconciliation failed")
	g.Expect(res).To(Equal(ctrl.Result{}))

	g.Expect(c.Get(context.Background(), client.ObjectKeyFromObject(imagePol), imagePol)).
		To(Succeed(), "failed getting image policy")

	g.Expect(imagePol.Status.LatestImage).To(Equal("ghcr.io/stefanprodan/podinfo:1.0.0"), "unexpected latest image")
	g.Expect(imagePol.Status.LatestRef).To(Equal(&imagev1.ImageRef{
		Name:   "ghcr.io/stefanprodan/podinfo",
		Tag:    "1.0.0",
		Digest: "",
	}), "unexpected latest ref")
	g.Expect(imagePol.Status.ObservedPreviousImage).To(BeEmpty(), "unexpected observed previous image")
	g.Expect(imagePol.Status.ObservedPreviousRef).To(BeNil(), "unexpected observed previous ref")
}

func TestImagePolicyReconciler_getImageRepository(t *testing.T) {
	testImageRepoName := "test-repo"
	testNamespace1 := "test-ns1" // Default namespace of ImagePolicy.
	testNamespace2 := "test-ns2" // Used for cross-namespace repo reference.

	tests := []struct {
		name                  string
		aclOpts               acl.Options
		imagePolicySpec       imagev1.ImagePolicySpec
		policyNamespaceLabels map[string]string
		imageRepoNamespace    string
		imageRepoAccessFrom   *aclapis.AccessFrom
		wantErr               bool
		wantRepo              string
	}{
		{
			name:    "NoCrossNamespaceRefs=true, repo in same namespace",
			aclOpts: acl.Options{NoCrossNamespaceRefs: true},
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name: testImageRepoName,
				},
			},
			imageRepoNamespace: testNamespace1,
			wantRepo:           testImageRepoName,
		},
		{
			name:    "NoCrossNamespaceRefs=true, repo in different namespace",
			aclOpts: acl.Options{NoCrossNamespaceRefs: true},
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name:      testImageRepoName,
					Namespace: testNamespace2,
				},
			},
			imageRepoNamespace: testNamespace2,
			wantErr:            true,
		},
		{
			name: "referred repo does not exist",
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name: "some-non-existing-repo",
				},
			},
			wantErr: true,
		},
		{
			name: "repo in same namespace",
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name: testImageRepoName,
				},
			},
			imageRepoNamespace: testNamespace1,
			wantRepo:           testImageRepoName,
		},
		{
			name: "repo in different namespace, ACL not authorized",
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name:      testImageRepoName,
					Namespace: testNamespace2,
				},
			},
			policyNamespaceLabels: map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			imageRepoNamespace: testNamespace2,
			wantErr:            true,
		},
		{
			name: "repo in different namespace, ACL authorized",
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name:      testImageRepoName,
					Namespace: testNamespace2,
				},
			},
			policyNamespaceLabels: map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			imageRepoNamespace: testNamespace2,
			imageRepoAccessFrom: &aclapis.AccessFrom{
				NamespaceSelectors: []aclapis.NamespaceSelector{
					{MatchLabels: map[string]string{"foo1": "bar1"}},
				},
			},
			wantRepo: testImageRepoName,
		},
		{
			name: "repo in different namespace, multiple ACL namespace selectors, authorized",
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name:      testImageRepoName,
					Namespace: testNamespace2,
				},
			},
			policyNamespaceLabels: map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			imageRepoNamespace: testNamespace2,
			imageRepoAccessFrom: &aclapis.AccessFrom{
				NamespaceSelectors: []aclapis.NamespaceSelector{
					{MatchLabels: map[string]string{"aaa": "bbb"}},
					{MatchLabels: map[string]string{"foo2": "bar2"}},
					{MatchLabels: map[string]string{"xxx": "yyy"}},
				},
			},
			wantRepo: testImageRepoName,
		},
		{
			name: "repo in different namespace, multiple ACL namespace selectors, unauthorized",
			imagePolicySpec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name:      testImageRepoName,
					Namespace: testNamespace2,
				},
			},
			policyNamespaceLabels: map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			},
			imageRepoNamespace: testNamespace2,
			imageRepoAccessFrom: &aclapis.AccessFrom{
				NamespaceSelectors: []aclapis.NamespaceSelector{
					{MatchLabels: map[string]string{"aaa": "bbb"}},
					{MatchLabels: map[string]string{"mmm": "nnn"}},
					{MatchLabels: map[string]string{"xxx": "yyy"}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create namespace where ImagePolicy exists.
			imagePolicyNS := &corev1.Namespace{}
			imagePolicyNS.Name = testNamespace1
			if tt.policyNamespaceLabels != nil {
				imagePolicyNS.SetLabels(tt.policyNamespaceLabels)
			}

			// Create a second namespace for cross-namespace reference of
			// ImageRepository if needed.
			imageRepoNS := &corev1.Namespace{}
			imageRepoNS.Name = testNamespace2

			// Create ImageRepository.
			imageRepo := &imagev1.ImageRepository{}
			imageRepo.Name = testImageRepoName
			imageRepo.Namespace = tt.imageRepoNamespace
			if tt.imageRepoAccessFrom != nil {
				imageRepo.Spec.AccessFrom = tt.imageRepoAccessFrom
			}

			clientBuilder := fake.NewClientBuilder()
			clientBuilder.WithObjects(imagePolicyNS, imageRepoNS, imageRepo)

			r := &ImagePolicyReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Client:        clientBuilder.Build(),
				ACLOptions:    tt.aclOpts,
				patchOptions:  getPatchOptions(imagePolicyOwnedConditions, "irc"),
			}

			obj := &imagev1.ImagePolicy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "reconcile-policy-",
					Generation:   1,
					Namespace:    testNamespace1,
				},
			}
			obj.Spec = tt.imagePolicySpec

			repo, err := r.getImageRepository(context.TODO(), obj)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if err == nil {
				g.Expect(repo.Name).To(Equal(tt.wantRepo))
			}
		})
	}
}

func TestImagePolicyReconciler_digestReflection(t *testing.T) {
	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	versions := []string{"v1.0.0", "v1.1.0", "v1.1.1", "v2.0.0"}
	imgRepo, images1stPass, err := test.LoadImages(registryServer, "foo/bar", versions)
	if err != nil {
		t.Fatalf("could not load images into test registry: %s", err)
	}

	var images2ndPass map[string]v1.Hash

	tests := []struct {
		name                string
		semVerPolicy2ndPass string
		refPolicy1stPass    imagev1.ReflectionPolicy
		refPolicy2ndPass    imagev1.ReflectionPolicy
		digest1stPass       func() string
		digest2ndPass       func() string
		previousRef2ndPass  func() *imagev1.ImageRef
	}{
		{
			name:             "missing policy leaves digest empty",
			refPolicy1stPass: "",
			digest1stPass: func() string {
				return ""
			},
			digest2ndPass: func() string {
				return ""
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return nil
			},
		},
		{
			name:             "'Never' policy leaves digest empty",
			refPolicy1stPass: imagev1.ReflectNever,
			digest1stPass: func() string {
				return ""
			},
			digest2ndPass: func() string {
				return ""
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return nil
			},
		},
		{
			name:             "'Always' policy always updates digest",
			refPolicy1stPass: imagev1.ReflectAlways,
			refPolicy2ndPass: imagev1.ReflectAlways,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return images2ndPass["v1.1.1"].String()
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return &imagev1.ImageRef{
					Name:   imgRepo,
					Tag:    "v1.1.1",
					Digest: images1stPass["v1.1.1"].String(),
				}
			},
		},
		{
			name:                "'IfNotPresent' policy updates digest when new tag is selected",
			semVerPolicy2ndPass: "v2.x",
			refPolicy1stPass:    imagev1.ReflectIfNotPresent,
			refPolicy2ndPass:    imagev1.ReflectIfNotPresent,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return images2ndPass["v2.0.0"].String()
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return &imagev1.ImageRef{
					Name:   imgRepo,
					Tag:    "v1.1.1",
					Digest: images1stPass["v1.1.1"].String(),
				}
			},
		},
		{
			name:             "'IfNotPresent' policy only sets digest once",
			refPolicy1stPass: imagev1.ReflectIfNotPresent,
			refPolicy2ndPass: imagev1.ReflectIfNotPresent,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return nil
			},
		},
		{
			name:             "changing 'Never' to 'IfNotPresent' sets observedPreviousRef correctly",
			refPolicy1stPass: imagev1.ReflectNever,
			refPolicy2ndPass: imagev1.ReflectIfNotPresent,
			digest1stPass: func() string {
				return ""
			},
			digest2ndPass: func() string {
				return images2ndPass["v1.1.1"].String()
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return &imagev1.ImageRef{
					Name:   imgRepo,
					Tag:    "v1.1.1",
					Digest: "",
				}
			},
		},
		{
			name:             "unsetting 'Always' policy removes digest",
			refPolicy1stPass: imagev1.ReflectAlways,
			refPolicy2ndPass: imagev1.ReflectNever,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return ""
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return &imagev1.ImageRef{
					Name:   imgRepo,
					Tag:    "v1.1.1",
					Digest: images1stPass["v1.1.1"].String(),
				}
			},
		},
		{
			name:             "unsetting 'IfNotPresent' policy removes digest",
			refPolicy1stPass: imagev1.ReflectIfNotPresent,
			refPolicy2ndPass: imagev1.ReflectNever,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return ""
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return &imagev1.ImageRef{
					Name:   imgRepo,
					Tag:    "v1.1.1",
					Digest: images1stPass["v1.1.1"].String(),
				}
			},
		},
		{
			name:             "changing 'IfNotPresent' to 'Always' sets new digest",
			refPolicy1stPass: imagev1.ReflectIfNotPresent,
			refPolicy2ndPass: imagev1.ReflectAlways,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return images2ndPass["v1.1.1"].String()
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return &imagev1.ImageRef{
					Name:   imgRepo,
					Tag:    "v1.1.1",
					Digest: images1stPass["v1.1.1"].String(),
				}
			},
		},
		{
			name:             "changing 'Always' to 'IfNotPresent' leaves digest untouched",
			refPolicy1stPass: imagev1.ReflectAlways,
			refPolicy2ndPass: imagev1.ReflectIfNotPresent,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return nil
			},
		},
		{
			name:                "selecting same tag with different policy leaves observedPreviousRef empty",
			refPolicy1stPass:    imagev1.ReflectIfNotPresent,
			semVerPolicy2ndPass: "=v1.1.1",
			refPolicy2ndPass:    imagev1.ReflectIfNotPresent,
			digest1stPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			digest2ndPass: func() string {
				return images1stPass["v1.1.1"].String()
			},
			previousRef2ndPass: func() *imagev1.ImageRef {
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			g := NewWithT(t)

			// Create namespace where ImagePolicy exists.
			ns := &corev1.Namespace{}
			ns.Name = "digref-test"

			// Create ImageRepository.
			imageRepo := &imagev1.ImageRepository{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns.Name,
					Name:      "digref-test",
				},
				Spec: imagev1.ImageRepositorySpec{
					Image: imgRepo,
				},
				Status: imagev1.ImageRepositoryStatus{
					LastScanResult: &imagev1.ScanResult{
						TagCount:   len(versions),
						LatestTags: versions,
					},
				},
			}

			imagePol := &imagev1.ImagePolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  ns.Name,
					Name:       "digref-test",
					Finalizers: []string{imagev1.ImageFinalizer},
				},
				Spec: imagev1.ImagePolicySpec{
					ImageRepositoryRef: meta.NamespacedObjectReference{
						Name: imageRepo.Name,
					},
					DigestReflectionPolicy: tt.refPolicy1stPass,
					Policy: imagev1.ImagePolicyChoice{
						SemVer: &imagev1.SemVerPolicy{
							Range: "v1.x",
						},
					},
				},
			}

			s := runtime.NewScheme()
			utilruntime.Must(imagev1.AddToScheme(s))
			utilruntime.Must(corev1.AddToScheme(s))

			c := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(ns, imageRepo, imagePol).
				WithStatusSubresource(imagePol).
				Build()

			g.Expect(
				c.Get(context.Background(), client.ObjectKeyFromObject(imageRepo), imageRepo),
			).To(Succeed(), "failed getting image repo")

			r := &ImagePolicyReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Client:        c,
				Database:      &mockDatabase{TagData: imageRepo.Status.LastScanResult.LatestTags},
				AuthOptionsGetter: registry.NewAuthOptionsGetter(c, login.ProviderOptions{
					AwsAutoLogin:   false,
					AzureAutoLogin: false,
					GcpAutoLogin:   false,
				}),
			}

			res, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ns.Name,
					Name:      imagePol.Name,
				},
			})

			g.Expect(err).NotTo(HaveOccurred(), "reconciliation failed")
			g.Expect(res).To(Equal(ctrl.Result{}))

			g.Expect(
				c.Get(context.Background(), client.ObjectKeyFromObject(imagePol), imagePol),
			).To(Succeed(), "failed getting image policy")

			g.Expect(imagePol.Status.LatestRef.Digest).
				To(Equal(tt.digest1stPass()), "unexpected 1st pass digest in status")
			g.Expect(imagePol.Status.ObservedPreviousRef).To(BeNil(),
				"observedPreviousRef should always be nil after a single pass")

			// Now, change the policy (if the test desires it) and overwrite the existing latest tag with a new image

			if tt.refPolicy1stPass != tt.refPolicy2ndPass {
				imagePol.Spec.DigestReflectionPolicy = tt.refPolicy2ndPass
			}
			if tt.semVerPolicy2ndPass != "" {
				imagePol.Spec.Policy.SemVer.Range = tt.semVerPolicy2ndPass
			}

			g.Expect(
				c.Update(context.Background(), imagePol),
			).To(Succeed(), "failed updating image policy for 2nd pass")

			if _, images2ndPass, err = test.LoadImages(registryServer, "foo/bar", versions); err != nil {
				t.Fatalf("could not overwrite tag: %s", err)
			}

			defer func() {
				// the new 1st pass is the old 2nd pass in the next sub-test
				images1stPass = images2ndPass
			}()

			res, err = r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ns.Name,
					Name:      imagePol.Name,
				},
			})

			g.Expect(err).NotTo(HaveOccurred(), "reconciliation failed")
			g.Expect(res).To(Equal(ctrl.Result{}))

			g.Expect(
				c.Get(context.Background(), client.ObjectKeyFromObject(imagePol), imagePol),
			).To(Succeed(), "failed getting image policy")
			g.Expect(imagePol.Status.LatestRef.Digest).
				To(Equal(tt.digest2ndPass()), "unexpected 2nd pass digest in status")
			g.Expect(imagePol.Status.ObservedPreviousRef).To(Equal(tt.previousRef2ndPass()),
				"unexpected content in .status.observedPreviousRef")
		})
	}
}

func TestImagePolicyReconciler_applyPolicy(t *testing.T) {
	tests := []struct {
		name       string
		policy     imagev1.ImagePolicyChoice
		filter     *imagev1.TagFilter
		db         *mockDatabase
		wantErr    bool
		wantResult string
	}{
		{
			name:    "invalid policy",
			policy:  imagev1.ImagePolicyChoice{},
			wantErr: true,
		},
		{
			name:    "database read fail",
			policy:  imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "1.0.x"}},
			db:      &mockDatabase{ReadError: errors.New("fail")},
			wantErr: true,
		},
		{
			name:    "no tags in database",
			policy:  imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "1.0.x"}},
			db:      &mockDatabase{},
			wantErr: true,
		},
		{
			name:       "semver, no tag filter",
			policy:     imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "1.0.x"}},
			db:         &mockDatabase{TagData: []string{"1.0.0", "2.0.0", "1.0.1", "1.2.0"}},
			wantResult: "1.0.1",
		},
		{
			name:       "semver with 'v' prefix, no tag filter",
			policy:     imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "v1.0.x"}},
			db:         &mockDatabase{TagData: []string{"v1.0.0", "v2.0.0", "v1.0.1", "v1.2.0"}},
			wantResult: "v1.0.1",
		},
		{
			name:       "semver with 'v' prefix but data without 'v' prefix, no tag filter",
			policy:     imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "v1.0.x"}},
			db:         &mockDatabase{TagData: []string{"1.0.0", "2.0.0", "1.0.1", "1.2.0"}},
			wantResult: "1.0.1",
		},
		{
			name:       "semver without 'v' prefix but data with 'v' prefix, no tag filter",
			policy:     imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "1.0.x"}},
			db:         &mockDatabase{TagData: []string{"v1.0.0", "v2.0.0", "v1.0.1", "v1.2.0"}},
			wantResult: "v1.0.1",
		},
		{
			name:    "invalid tag filter",
			policy:  imagev1.ImagePolicyChoice{SemVer: &imagev1.SemVerPolicy{Range: "1.0.x"}},
			filter:  &imagev1.TagFilter{Pattern: "[="},
			db:      &mockDatabase{TagData: []string{"1.0.0", "1.0.1"}},
			wantErr: true,
		},
		{
			name:   "valid tag filter with numerical policy",
			policy: imagev1.ImagePolicyChoice{Numerical: &imagev1.NumericalPolicy{Order: policy.NumericalOrderAsc}},
			filter: &imagev1.TagFilter{
				Pattern: "1.0.0-rc\\.(?P<num>[0-9]+)",
				Extract: "$num",
			},
			db: &mockDatabase{TagData: []string{
				"1.0.0", "1.0.0-rc.1", "1.0.0-rc.2", "1.0.0-rc.3", "1.0.1-rc.2",
			}},
			wantResult: "1.0.0-rc.3",
		},
		{
			name:   "valid tag filter with alphabetical policy",
			policy: imagev1.ImagePolicyChoice{Alphabetical: &imagev1.AlphabeticalPolicy{Order: policy.AlphabeticalOrderAsc}},
			filter: &imagev1.TagFilter{
				Pattern: "foo-(?P<word>[a-z]+)",
				Extract: "$word",
			},
			db: &mockDatabase{TagData: []string{
				"foo-aaa", "bar-bbb", "foo-zzz", "baz-nnn", "foo-ooo",
			}},
			wantResult: "foo-zzz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			r := &ImagePolicyReconciler{
				EventRecorder: record.NewFakeRecorder(32),
				Database:      tt.db,
				patchOptions:  getPatchOptions(imagePolicyOwnedConditions, "irc"),
			}

			obj := &imagev1.ImagePolicy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "reconcile-policy-",
					Generation:   1,
				},
			}
			obj.Spec.Policy = tt.policy
			obj.Spec.FilterTags = tt.filter

			repo := &imagev1.ImageRepository{}

			result, err := r.applyPolicy(context.TODO(), obj, repo)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if err == nil {
				g.Expect(result).To(Equal(tt.wantResult))
			}
		})
	}
}

func TestComposeImagePolicyReadyMessage(t *testing.T) {
	testImage := "foo/bar"

	tests := []struct {
		name        string
		previousTag string
		latestTag   string
		image       string
		wantMessage string
	}{
		{
			name:        "no previous tag",
			latestTag:   "1.0.0",
			wantMessage: "Latest image tag for 'foo/bar' resolved to 1.0.0",
		},
		{
			name:        "different previous tag",
			previousTag: "1.0.0",
			latestTag:   "1.1.0",
			wantMessage: "Latest image tag for 'foo/bar' updated from 1.0.0 to 1.1.0",
		},
		{
			name:        "same previous and latest tags",
			previousTag: "1.0.0",
			latestTag:   "1.0.0",
			wantMessage: "Latest image tag for 'foo/bar' resolved to 1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := composeImagePolicyReadyMessage(tt.previousTag, tt.latestTag, testImage)
			g.Expect(result).To(Equal(tt.wantMessage))
		})
	}
}
