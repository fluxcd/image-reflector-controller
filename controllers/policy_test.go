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

package controllers

import (
	"context"
	"net/http/httptest"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	aclapi "github.com/fluxcd/pkg/apis/acl"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	"github.com/fluxcd/image-reflector-controller/internal/test"
	// +kubebuilder:scaffold:imports
)

// https://github.com/google/go-containerregistry/blob/v0.1.1/pkg/registry/compatibility_test.go
// has an example of loading a test registry with a random image.

var _ = Describe("ImagePolicy controller", func() {

	var registryServer *httptest.Server

	BeforeEach(func() {
		registryServer = test.NewRegistryServer()
	})

	AfterEach(func() {
		registryServer.Close()
	})

	When("cross-namespace refs disallowed", func() {
		BeforeEach(func() {
			imagePolicyReconciler.ACLOptions.NoCrossNamespaceRefs = true
		})

		AfterEach(func() {
			imagePolicyReconciler.ACLOptions.NoCrossNamespaceRefs = false
		})

		It("fails to reconcile an ImagePolicy with a cross-ns ref", func() {
			// a bona fide image repo is needed so that it _would_ succeed if not for the disallowed cross-ns ref.
			versions := []string{"1.0.1", "1.0.2", "1.1.0-alpha"}
			imgRepo, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), versions)
			Expect(err).ToNot(HaveOccurred())

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
			Expect(k8sClient.Create(ctx, &repo)).To(Succeed())

			ns := corev1.Namespace{}
			ns.Name = "cross-ns-test-" + randStringRunes(5)
			Expect(k8sClient.Create(ctx, &ns)).To(Succeed())

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
			Expect(k8sClient.Create(ctx, &imagePolicy)).To(Succeed())

			var pol imagev1.ImagePolicy
			Eventually(func() bool {
				err := k8sClient.Get(ctx, imagePolicyName, &pol)
				return err == nil && apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
			}, timeout, interval).Should(BeTrue())
			ready := apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition)
			Expect(ready.Reason).To(Equal(aclapi.AccessDeniedReason))
		})
	})

	Context("Calculates an image from a repository's tags", func() {
		When("Using SemVerPolicy", func() {
			It("calculates an image from a repository's tags", func() {
				versions := []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
				imgRepo, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())

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

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, imageObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polName := types.NamespacedName{
					Name:      "random-pol-" + randStringRunes(5),
					Namespace: imageObjectName.Namespace,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name: imageObjectName.Name,
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

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":1.0.2"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})

		When("Using SemVerPolicy with invalid range", func() {
			It("fails with invalid policy error", func() {
				versions := []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
				imgRepo, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())

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

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, imageObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polName := types.NamespacedName{
					Name:      "random-pol-" + randStringRunes(5),
					Namespace: imageObjectName.Namespace,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name: imageObjectName.Name,
						},
						Policy: imagev1.ImagePolicyChoice{
							SemVer: &imagev1.SemVerPolicy{
								Range: "*-*",
							},
						},
					},
				}
				pol.Namespace = polName.Namespace
				pol.Name = polName.Name

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polName, &pol)
					return err == nil && apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
				}, timeout, interval).Should(BeTrue())
				ready := apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition)
				Expect(ready.Reason).To(ContainSubstring("InvalidPolicy"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})

		When("Usign AlphabeticalPolicy", func() {
			It("calculates an image from a repository's tags", func() {
				versions := []string{"xenial", "yakkety", "zesty", "artful", "bionic"}
				imgRepo, err := test.LoadImages(registryServer, "test-alphabetical-policy-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())

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

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, imageObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polName := types.NamespacedName{
					Name:      "random-pol-" + randStringRunes(5),
					Namespace: imageObjectName.Namespace,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name: imageObjectName.Name,
						},
						Policy: imagev1.ImagePolicyChoice{
							Alphabetical: &imagev1.AlphabeticalPolicy{},
						},
					},
				}
				pol.Namespace = polName.Namespace
				pol.Name = polName.Name

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":zesty"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})
	})

	Context("Filters tags", func() {
		When("valid regex supplied", func() {
			It("correctly filters the repo tags", func() {
				versions := []string{"test-0.1.0", "test-0.1.1", "dev-0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
				imgRepo, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())
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

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, imageObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polName := types.NamespacedName{
					Name:      "random-pol-" + randStringRunes(5),
					Namespace: imageObjectName.Namespace,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name: imageObjectName.Name,
						},
						FilterTags: &imagev1.TagFilter{
							Pattern: "^test-(.*)$",
							Extract: "$1",
						},
						Policy: imagev1.ImagePolicyChoice{
							SemVer: &imagev1.SemVerPolicy{
								Range: ">=0.x",
							},
						},
					},
				}
				pol.Namespace = polName.Namespace
				pol.Name = polName.Name

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":test-0.1.1"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})

		When("invalid regex supplied", func() {
			It("fails to reconcile returning error", func() {
				versions := []string{"test-0.1.0", "test-0.1.1", "dev-0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
				imgRepo, err := test.LoadImages(registryServer, "test-semver-policy-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())
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

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, imageObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polName := types.NamespacedName{
					Name:      "random-pol-" + randStringRunes(5),
					Namespace: imageObjectName.Namespace,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name: imageObjectName.Name,
						},
						FilterTags: &imagev1.TagFilter{
							Pattern: "^test-(.*",
							Extract: "$1",
						},
						Policy: imagev1.ImagePolicyChoice{
							SemVer: &imagev1.SemVerPolicy{
								Range: ">=0.x",
							},
						},
					},
				}
				pol.Namespace = polName.Namespace
				pol.Name = polName.Name

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				// Currently succeeds creating the resources as there's no
				// admission webhook validation
				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polName, &pol)
					return err == nil && apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
				}, timeout, interval).Should(BeTrue())

				ready := apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition)
				Expect(ready.Message).To(ContainSubstring("invalid regular expression pattern"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})
	})

	Context("Access ImageRepository", func() {
		When("is in same namespace", func() {
			It("grants access", func() {
				versions := []string{"1.0.0", "1.0.1"}
				imageName := "test-acl-" + randStringRunes(5)
				imgRepo, err := test.LoadImages(registryServer, imageName, versions)
				Expect(err).ToNot(HaveOccurred())

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

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, imageObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polName := types.NamespacedName{
					Name:      "random-pol-" + randStringRunes(5),
					Namespace: imageObjectName.Namespace,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name: imageObjectName.Name,
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

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":1.0.1"))

				// Updating the image should reconcile the cross-namespace policy
				imgRepo, err = test.LoadImages(registryServer, imageName, []string{"1.0.2"})
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					err := r.Get(ctx, imageObjectName, &repo)
					return err == nil && repo.Status.LastScanResult.TagCount == len(versions)+1
				}, timeout, interval).Should(BeTrue())

				Eventually(func() bool {
					err := r.Get(ctx, polName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":1.0.2"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})

		When("is in different namespace with empty ACL", func() {
			It("deny access", func() {
				policyNamespace := &corev1.Namespace{}
				policyNamespace.Name = "acl-" + randStringRunes(5)
				policyNamespace.Labels = map[string]string{
					"tenant": "a",
					"env":    "test",
				}
				Expect(k8sClient.Create(context.Background(), policyNamespace)).To(Succeed())
				defer k8sClient.Delete(context.Background(), policyNamespace)

				versions := []string{"1.0.0", "1.0.1"}
				imgRepo, err := test.LoadImages(registryServer, "acl-image-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())

				repo := imagev1.ImageRepository{
					Spec: imagev1.ImageRepositorySpec{
						Interval:   metav1.Duration{Duration: reconciliationInterval},
						Image:      imgRepo,
						AccessFrom: nil,
					},
				}
				repoObjectName := types.NamespacedName{
					Name:      "acl-repo-" + randStringRunes(5),
					Namespace: "default",
				}
				repo.Name = repoObjectName.Name
				repo.Namespace = repoObjectName.Namespace

				ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, repoObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polObjectName := types.NamespacedName{
					Name:      "acl-pol-" + randStringRunes(5),
					Namespace: policyNamespace.Name,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name:      repoObjectName.Name,
							Namespace: repoObjectName.Namespace,
						},
						Policy: imagev1.ImagePolicyChoice{
							SemVer: &imagev1.SemVerPolicy{
								Range: "1.0.x",
							},
						},
					},
				}
				pol.Namespace = polObjectName.Namespace
				pol.Name = polObjectName.Name

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					_ = r.Get(ctx, polObjectName, &pol)
					return apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
				}, timeout, interval).Should(BeTrue())
				Expect(apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition).Reason).To(Equal(aclapi.AccessDeniedReason))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})

		When("is in different namespace with empty match labels", func() {
			It("grants access", func() {
				policyNamespace := &corev1.Namespace{}
				policyNamespace.Name = "acl-" + randStringRunes(5)

				Expect(k8sClient.Create(context.Background(), policyNamespace)).To(Succeed())
				defer k8sClient.Delete(context.Background(), policyNamespace)

				versions := []string{"1.0.0", "1.0.1"}
				imgRepo, err := test.LoadImages(registryServer, "acl-image-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())

				repo := imagev1.ImageRepository{
					Spec: imagev1.ImageRepositorySpec{
						Interval: metav1.Duration{Duration: reconciliationInterval},
						Image:    imgRepo,
						AccessFrom: &aclapi.AccessFrom{
							NamespaceSelectors: []aclapi.NamespaceSelector{
								{
									MatchLabels: make(map[string]string),
								},
							},
						},
					},
				}
				repoObjectName := types.NamespacedName{
					Name:      "acl-repo-" + randStringRunes(5),
					Namespace: "default",
				}
				repo.Name = repoObjectName.Name
				repo.Namespace = repoObjectName.Namespace

				ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, repoObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polObjectName := types.NamespacedName{
					Name:      "acl-pol-" + randStringRunes(5),
					Namespace: policyNamespace.Name,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name:      repoObjectName.Name,
							Namespace: repoObjectName.Namespace,
						},
						Policy: imagev1.ImagePolicyChoice{
							SemVer: &imagev1.SemVerPolicy{
								Range: "1.0.x",
							},
						},
					},
				}
				pol.Namespace = polObjectName.Namespace
				pol.Name = polObjectName.Name

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polObjectName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":1.0.1"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})

		When("is in different namespace with matching ACL", func() {
			It("grants access", func() {
				policyNamespace := &corev1.Namespace{}
				policyNamespace.Name = "acl-" + randStringRunes(5)
				policyNamespace.Labels = map[string]string{
					"tenant": "a",
					"env":    "test",
				}
				Expect(k8sClient.Create(context.Background(), policyNamespace)).To(Succeed())
				defer k8sClient.Delete(context.Background(), policyNamespace)

				versions := []string{"1.0.0", "1.0.1"}
				imageName := "acl-image-" + randStringRunes(5)
				imgRepo, err := test.LoadImages(registryServer, imageName, versions)
				Expect(err).ToNot(HaveOccurred())

				repo := imagev1.ImageRepository{
					Spec: imagev1.ImageRepositorySpec{
						Interval: metav1.Duration{Duration: reconciliationInterval},
						Image:    imgRepo,
						AccessFrom: &aclapi.AccessFrom{
							NamespaceSelectors: []aclapi.NamespaceSelector{
								{
									MatchLabels: policyNamespace.Labels,
								},
								{
									MatchLabels: map[string]string{
										"tenant": "b",
										"env":    "test",
									},
								},
							},
						},
					},
				}
				repoObjectName := types.NamespacedName{
					Name:      "acl-repo-" + randStringRunes(5),
					Namespace: "default",
				}
				repo.Name = repoObjectName.Name
				repo.Namespace = repoObjectName.Namespace

				ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, repoObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polObjectName := types.NamespacedName{
					Name:      "acl-pol-" + randStringRunes(5),
					Namespace: policyNamespace.Name,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name:      repoObjectName.Name,
							Namespace: repoObjectName.Namespace,
						},
						Policy: imagev1.ImagePolicyChoice{
							SemVer: &imagev1.SemVerPolicy{
								Range: "1.0.x",
							},
						},
					},
				}
				pol.Namespace = polObjectName.Namespace
				pol.Name = polObjectName.Name

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, polObjectName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":1.0.1"))

				// Updating the image should reconcile the cross-namespace policy
				imgRepo, err = test.LoadImages(registryServer, imageName, []string{"1.0.2"})
				Expect(err).ToNot(HaveOccurred())
				Eventually(func() bool {
					err := r.Get(ctx, repoObjectName, &repo)
					return err == nil && repo.Status.LastScanResult.TagCount == len(versions)+1
				}, timeout, interval).Should(BeTrue())

				Eventually(func() bool {
					err := r.Get(ctx, polObjectName, &pol)
					return err == nil && pol.Status.LatestImage != ""
				}, timeout, interval).Should(BeTrue())
				Expect(pol.Status.LatestImage).To(Equal(imgRepo + ":1.0.2"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})

		When("is in different namespace with mismatching ACL", func() {
			It("denies access", func() {
				policyNamespace := &corev1.Namespace{}
				policyNamespace.Name = "acl-" + randStringRunes(5)
				policyNamespace.Labels = map[string]string{
					"tenant": "a",
					"env":    "test",
				}
				Expect(k8sClient.Create(context.Background(), policyNamespace)).To(Succeed())
				defer k8sClient.Delete(context.Background(), policyNamespace)

				versions := []string{"1.0.0", "1.0.1"}
				imgRepo, err := test.LoadImages(registryServer, "acl-image-"+randStringRunes(5), versions)
				Expect(err).ToNot(HaveOccurred())

				repo := imagev1.ImageRepository{
					Spec: imagev1.ImageRepositorySpec{
						Interval: metav1.Duration{Duration: reconciliationInterval},
						Image:    imgRepo,
						AccessFrom: &aclapi.AccessFrom{
							NamespaceSelectors: []aclapi.NamespaceSelector{
								{
									MatchLabels: map[string]string{
										"tenant": "b",
										"env":    "test",
									},
								},
							},
						},
					},
				}
				repoObjectName := types.NamespacedName{
					Name:      "acl-repo-" + randStringRunes(5),
					Namespace: "default",
				}
				repo.Name = repoObjectName.Name
				repo.Namespace = repoObjectName.Namespace

				ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				r := imageRepoReconciler
				Expect(r.Create(ctx, &repo)).To(Succeed())

				Eventually(func() bool {
					err := r.Get(ctx, repoObjectName, &repo)
					return err == nil && repo.Status.LastScanResult != nil
				}, timeout, interval).Should(BeTrue())
				Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
				Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

				polObjectName := types.NamespacedName{
					Name:      "acl-pol-" + randStringRunes(5),
					Namespace: policyNamespace.Name,
				}
				pol := imagev1.ImagePolicy{
					Spec: imagev1.ImagePolicySpec{
						ImageRepositoryRef: meta.NamespacedObjectReference{
							Name:      repoObjectName.Name,
							Namespace: repoObjectName.Namespace,
						},
						Policy: imagev1.ImagePolicyChoice{
							SemVer: &imagev1.SemVerPolicy{
								Range: "1.0.x",
							},
						},
					},
				}
				pol.Namespace = polObjectName.Namespace
				pol.Name = polObjectName.Name

				ctx, cancel = context.WithTimeout(context.Background(), contextTimeout)
				defer cancel()

				Expect(r.Create(ctx, &pol)).To(Succeed())

				Eventually(func() bool {
					_ = r.Get(ctx, polObjectName, &pol)
					return apimeta.IsStatusConditionFalse(pol.Status.Conditions, meta.ReadyCondition)
				}, timeout, interval).Should(BeTrue())
				Expect(apimeta.FindStatusCondition(pol.Status.Conditions, meta.ReadyCondition).Reason).To(Equal("AccessDenied"))

				Expect(r.Delete(ctx, &pol)).To(Succeed())
			})
		})
	})
})
