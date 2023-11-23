/*
Copyright 2020 The Flux authors

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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	aclapi "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/conditions"
	conditionscheck "github.com/fluxcd/pkg/runtime/conditions/check"
	"github.com/fluxcd/pkg/runtime/patch"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/database"
	"github.com/fluxcd/image-reflector-controller/internal/test"
	// +kubebuilder:scaffold:imports
)

// https://github.com/google/go-containerregistry/blob/v0.1.1/pkg/registry/compatibility_test.go
// has an example of loading a test registry with a random image.

func TestImagePolicyReconciler_crossNamespaceRefsDisallowed(t *testing.T) {
	g := NewWithT(t)

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	versions := []string{"1.0.1", "1.0.2", "1.1.0-alpha"}
	imgRepo, _, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), versions)
	g.Expect(err).ToNot(HaveOccurred())

	namespaceLabels := map[string]string{
		"foo": "bar",
	}

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    imgRepo,
			AccessFrom: &aclapi.AccessFrom{
				NamespaceSelectors: []aclapi.NamespaceSelector{
					{
						MatchLabels: namespaceLabels,
					},
				},
			},
		},
	}
	imageObjectName := types.NamespacedName{
		Name:      "polimage-" + randStringRunes(5),
		Namespace: "default",
	}
	repo.Name = imageObjectName.Name
	repo.Namespace = imageObjectName.Namespace

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	ns := corev1.Namespace{}
	ns.Name = "cross-ns-test-" + randStringRunes(5)
	ns.Labels = namespaceLabels

	imagePolicyName := types.NamespacedName{
		Namespace: ns.Name,
		Name:      "policy-test-" + randStringRunes(5),
	}
	imagePolicy := imagev1.ImagePolicy{
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Namespace: repo.Namespace,
				Name:      repo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				SemVer: &imagev1.SemVerPolicy{
					Range: "1.x",
				},
			},
		},
	}
	imagePolicy.Namespace = imagePolicyName.Namespace
	imagePolicy.Name = imagePolicyName.Name

	builder := fakeclient.NewClientBuilder().WithScheme(testEnv.GetScheme())
	builder.WithObjects(&repo, &ns, &imagePolicy)

	r := &ImagePolicyReconciler{
		Client:        builder.Build(),
		Database:      database.NewBadgerDatabase(testBadgerDB),
		EventRecorder: record.NewFakeRecorder(32),
		ACLOptions: acl.Options{
			NoCrossNamespaceRefs: true,
		},
		patchOptions: getPatchOptions(imagePolicyOwnedConditions, "irc"),
	}

	sp := patch.NewSerialPatcher(&imagePolicy, r.Client)

	res, err := r.reconcile(ctx, sp, &imagePolicy)
	g.Expect(err).To(Not(BeNil()))
	g.Expect(res.Requeue).ToNot(BeTrue())
	g.Expect(conditions.GetReason(&imagePolicy, meta.ReadyCondition)).To(Equal(aclapi.AccessDeniedReason))
}

func TestImagePolicyReconciler_calculateImageFromRepoTags(t *testing.T) {
	tests := []struct {
		name         string
		versions     []string
		policy       imagev1.ImagePolicyChoice
		wantImageTag string
		wantFailure  bool
	}{
		{
			name:     "using SemVerPolicy",
			versions: []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"},
			policy: imagev1.ImagePolicyChoice{
				SemVer: &imagev1.SemVerPolicy{
					Range: "1.0.x",
				},
			},
			wantImageTag: ":1.0.2",
		},
		{
			name:     "using SemVerPolicy with invalid range",
			versions: []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"},
			policy: imagev1.ImagePolicyChoice{
				SemVer: &imagev1.SemVerPolicy{
					Range: "*-*",
				},
			},
			wantFailure: true,
		},
		{
			name:     "using AlphabeticalPolicy",
			versions: []string{"xenial", "yakkety", "zesty", "artful", "bionic"},
			policy: imagev1.ImagePolicyChoice{
				Alphabetical: &imagev1.AlphabeticalPolicy{},
			},
			wantImageTag: ":zesty",
		},
	}

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			imgRepo, _, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), tt.versions)
			g.Expect(err).ToNot(HaveOccurred())

			repo := imagev1.ImageRepository{
				Spec: imagev1.ImageRepositorySpec{
					Interval: metav1.Duration{Duration: reconciliationInterval},
					Image:    imgRepo,
				},
			}
			imageObjectName := types.NamespacedName{
				Name:      "polimage-" + randStringRunes(5),
				Namespace: "default",
			}
			repo.Name = imageObjectName.Name
			repo.Namespace = imageObjectName.Namespace

			ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
			defer cancel()

			g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

			g.Eventually(func() bool {
				err := testEnv.Get(ctx, imageObjectName, &repo)
				return err == nil && repo.Status.LastScanResult != nil
			}, timeout, interval).Should(BeTrue())
			g.Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
			g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(tt.versions)))

			polName := types.NamespacedName{
				Name:      "random-pol-" + randStringRunes(5),
				Namespace: imageObjectName.Namespace,
			}
			pol := imagev1.ImagePolicy{
				Spec: imagev1.ImagePolicySpec{
					ImageRepositoryRef: meta.NamespacedObjectReference{
						Name: imageObjectName.Name,
					},
					Policy: tt.policy,
				},
			}
			pol.Namespace = polName.Namespace
			pol.Name = polName.Name

			g.Expect(testEnv.Create(ctx, &pol)).To(Succeed())

			if !tt.wantFailure {
				g.Eventually(func() bool {
					err := testEnv.Get(ctx, polName, &pol)
					return err == nil &&
						pol.Status.LatestRef != nil
				}, timeout, interval).Should(BeTrue())
				g.Expect(pol.Status.LatestRef.String()).To(Equal(imgRepo + tt.wantImageTag))
				g.Expect(pol.Status.ObservedPreviousImage).To(Equal(""),
					"single reconciliation should leave status.observedPreviousImage empty")
				g.Expect(pol.Status.ObservedPreviousRef).To(BeNil(),
					"single reconciliation should leave status.observedPreviousRef nil")
			} else {
				g.Eventually(func() bool {
					err := testEnv.Get(ctx, polName, &pol)
					return err == nil && apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
				}, timeout, interval).Should(BeTrue())
				ready := apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition)
				g.Expect(ready.Reason).To(ContainSubstring("InvalidPolicy"))
			}

			// Check if the object status is valid.
			condns := &conditionscheck.Conditions{NegativePolarity: imagePolicyNegativeConditions}
			checker := conditionscheck.NewChecker(testEnv.Client, condns)
			checker.WithT(g).CheckErr(ctx, &pol)

			g.Expect(testEnv.Delete(ctx, &pol)).To(Succeed())
			g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
		})
	}
}

func TestImagePolicyReconciler_filterTags(t *testing.T) {
	tests := []struct {
		name         string
		versions     []string
		filterTags   *imagev1.TagFilter
		wantImageTag string
		wantFailure  bool
	}{
		{
			name:     "valid regex",
			versions: []string{"test-0.1.0", "test-0.1.1", "dev-0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"},
			filterTags: &imagev1.TagFilter{
				Pattern: "^test-(.*)$",
				Extract: "$1",
			},
			wantImageTag: ":test-0.1.1",
		},
		{
			name:     "invalid regex",
			versions: []string{"test-0.1.0", "test-0.1.1", "dev-0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"},
			filterTags: &imagev1.TagFilter{
				Pattern: "^test-(.*",
				Extract: "$1",
			},
			wantFailure: true,
		},
	}

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			imgRepo, _, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), tt.versions)
			g.Expect(err).ToNot(HaveOccurred())

			repo := imagev1.ImageRepository{
				Spec: imagev1.ImageRepositorySpec{
					Interval: metav1.Duration{Duration: reconciliationInterval},
					Image:    imgRepo,
				},
			}
			imageObjectName := types.NamespacedName{
				Name:      "polimage-" + randStringRunes(5),
				Namespace: "default",
			}
			repo.Name = imageObjectName.Name
			repo.Namespace = imageObjectName.Namespace

			ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
			defer cancel()

			g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

			g.Eventually(func() bool {
				err := testEnv.Get(ctx, imageObjectName, &repo)
				return err == nil && repo.Status.LastScanResult != nil
			}, timeout, interval).Should(BeTrue())
			g.Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
			g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(tt.versions)))

			polName := types.NamespacedName{
				Name:      "random-pol-" + randStringRunes(5),
				Namespace: imageObjectName.Namespace,
			}
			pol := imagev1.ImagePolicy{
				Spec: imagev1.ImagePolicySpec{
					ImageRepositoryRef: meta.NamespacedObjectReference{
						Name: imageObjectName.Name,
					},
					FilterTags: tt.filterTags,
					Policy: imagev1.ImagePolicyChoice{
						SemVer: &imagev1.SemVerPolicy{
							Range: ">=0.x",
						},
					},
				},
			}
			pol.Namespace = polName.Namespace
			pol.Name = polName.Name

			g.Expect(testEnv.Create(ctx, &pol)).To(Succeed())

			if !tt.wantFailure {
				g.Eventually(func() bool {
					err := testEnv.Get(ctx, polName, &pol)
					return err == nil && pol.Status.LatestRef != nil
				}, timeout, interval).Should(BeTrue())
				g.Expect(pol.Status.LatestRef.String()).To(Equal(imgRepo + tt.wantImageTag))
			} else {
				g.Eventually(func() bool {
					err := testEnv.Get(ctx, polName, &pol)
					return err == nil && apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
				}, timeout, interval).Should(BeTrue())
				ready := apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition)
				g.Expect(ready.Message).To(ContainSubstring("invalid regular expression pattern"))
			}

			// Check if the object status is valid.
			condns := &conditionscheck.Conditions{NegativePolarity: imagePolicyNegativeConditions}
			checker := conditionscheck.NewChecker(testEnv.Client, condns)
			checker.WithT(g).CheckErr(ctx, &pol)

			g.Expect(testEnv.Delete(ctx, &pol)).To(Succeed())
		})
	}
}

func TestImagePolicyReconciler_accessImageRepo(t *testing.T) {
	tests := []struct {
		name                       string
		imageRepoNamespace         string
		imageRepoAccessFrom        *aclapi.AccessFrom
		imagePolicyNamespace       string
		imagePolicyNamespaceLabels map[string]string
		wantAccessible             bool
	}{
		{
			name:                 "same namespace",
			imageRepoNamespace:   "default",
			imagePolicyNamespace: "default",
			wantAccessible:       true,
		},
		{
			name:                 "different namespaces, empty ACL",
			imageRepoNamespace:   "default",
			imageRepoAccessFrom:  nil,
			imagePolicyNamespace: "acl-" + randStringRunes(5),
			imagePolicyNamespaceLabels: map[string]string{
				"tenant": "a",
				"env":    "test",
			},
			wantAccessible: false,
		},
		{
			name:               "different namespaces, empty match labels",
			imageRepoNamespace: "default",
			imageRepoAccessFrom: &aclapi.AccessFrom{
				NamespaceSelectors: []aclapi.NamespaceSelector{
					{
						MatchLabels: make(map[string]string),
					},
				},
			},
			imagePolicyNamespace: "acl-" + randStringRunes(5),
			wantAccessible:       true,
		},
		{
			name:               "different namespaces, matching ACL",
			imageRepoNamespace: "default",
			imageRepoAccessFrom: &aclapi.AccessFrom{
				NamespaceSelectors: []aclapi.NamespaceSelector{
					{
						MatchLabels: map[string]string{
							"tenant": "b",
							"env":    "test",
						},
					},
				},
			},
			imagePolicyNamespace: "acl-" + randStringRunes(5),
			imagePolicyNamespaceLabels: map[string]string{
				"tenant": "b",
				"env":    "test",
			},
			wantAccessible: true,
		},
		{
			name:               "different namespaces, mismatching ACL",
			imageRepoNamespace: "default",
			imageRepoAccessFrom: &aclapi.AccessFrom{
				NamespaceSelectors: []aclapi.NamespaceSelector{
					{
						MatchLabels: map[string]string{
							"tenant": "b",
							"env":    "test",
						},
					},
				},
			},
			imagePolicyNamespace: "acl-" + randStringRunes(5),
			imagePolicyNamespaceLabels: map[string]string{
				"tenant": "a",
				"env":    "test",
			},
			wantAccessible: false,
		},
	}

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			versions := []string{"1.0.0", "1.0.1"}
			imgRepo, _, err := test.LoadImages(registryServer, "acl-image-"+randStringRunes(5), versions)
			g.Expect(err).ToNot(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
			defer cancel()

			// Create a new namespace if it's not the default one.
			if tt.imagePolicyNamespace != "default" {
				policyNamespace := &corev1.Namespace{}
				policyNamespace.Name = tt.imagePolicyNamespace
				policyNamespace.Labels = tt.imagePolicyNamespaceLabels
				g.Expect(testEnv.Create(ctx, policyNamespace)).To(Succeed())
				defer func() {
					g.Expect(testEnv.Delete(ctx, policyNamespace)).To(Succeed())
				}()
			}

			repo := imagev1.ImageRepository{
				Spec: imagev1.ImageRepositorySpec{
					Interval:   metav1.Duration{Duration: reconciliationInterval},
					Image:      imgRepo,
					AccessFrom: tt.imageRepoAccessFrom,
				},
			}
			imageObjectName := types.NamespacedName{
				Name:      "acl-repo-" + randStringRunes(5),
				Namespace: tt.imageRepoNamespace,
			}
			repo.Name = imageObjectName.Name
			repo.Namespace = imageObjectName.Namespace

			g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

			g.Eventually(func() bool {
				err := testEnv.Get(ctx, imageObjectName, &repo)
				return err == nil && repo.Status.LastScanResult != nil
			}, timeout, interval).Should(BeTrue())
			g.Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
			g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

			polName := types.NamespacedName{
				Name:      "acl-pol-" + randStringRunes(5),
				Namespace: tt.imagePolicyNamespace,
			}
			pol := imagev1.ImagePolicy{
				Spec: imagev1.ImagePolicySpec{
					ImageRepositoryRef: meta.NamespacedObjectReference{
						Name:      imageObjectName.Name,
						Namespace: tt.imageRepoNamespace,
					},
					Policy: imagev1.ImagePolicyChoice{
						SemVer: &imagev1.SemVerPolicy{
							Range: "1.0.x",
						},
					},
				},
			}
			pol.Namespace = polName.Namespace
			pol.Name = polName.Name

			g.Expect(testEnv.Create(ctx, &pol)).To(Succeed())

			if tt.wantAccessible {
				g.Eventually(func() bool {
					err := testEnv.Get(ctx, polName, &pol)
					return err == nil && pol.Status.LatestRef != nil
				}, timeout, interval).Should(BeTrue())
				g.Expect(pol.Status.LatestRef.String()).To(Equal(imgRepo + ":1.0.1"))
			} else {
				g.Eventually(func() bool {
					_ = testEnv.Get(ctx, polName, &pol)
					return apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
				}, timeout, interval).Should(BeTrue())
				g.Expect(apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition).Reason).To(Equal(aclapi.AccessDeniedReason))
			}

			// Check if the object status is valid.
			condns := &conditionscheck.Conditions{NegativePolarity: imagePolicyNegativeConditions}
			checker := conditionscheck.NewChecker(testEnv.Client, condns)
			checker.WithT(g).CheckErr(ctx, &pol)

			g.Expect(testEnv.Delete(ctx, &pol)).To(Succeed())
		})
	}
}
