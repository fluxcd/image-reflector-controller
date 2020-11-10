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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	imagev1alpha1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
	"github.com/fluxcd/pkg/apis/meta"
	// +kubebuilder:scaffold:imports
)

// https://github.com/google/go-containerregistry/blob/v0.1.1/pkg/registry/compatibility_test.go
// has an example of loading a test registry with a random image.

var _ = Describe("ImageRepository controller", func() {
	const imageName = "alpine-image"
	var repo imagev1alpha1.ImageRepository

	var registryServer *httptest.Server

	BeforeEach(func() {
		registryServer = newRegistryServer()
	})

	AfterEach(func() {
		registryServer.Close()
		Expect(k8sClient.Delete(context.Background(), &repo)).To(Succeed())
	})

	It("expands the canonical image name", func() {
		// would be good to test this without needing to do the scanning, since
		// 1. better to not rely on external services being available
		// 2. probably going to want to have several test cases
		repo = imagev1alpha1.ImageRepository{
			Spec: imagev1alpha1.ImageRepositorySpec{
				Image: "alpine",
			},
		}
		imageRepoName := types.NamespacedName{
			Name:      imageName,
			Namespace: "default",
		}

		repo.Name = imageRepoName.Name
		repo.Namespace = imageRepoName.Namespace

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		r := imageRepoReconciler
		err := r.Create(ctx, &repo)
		Expect(err).ToNot(HaveOccurred())

		var repoAfter imagev1alpha1.ImageRepository
		Eventually(func() bool {
			err := r.Get(context.Background(), imageRepoName, &repoAfter)
			return err == nil && repoAfter.Status.CanonicalImageName != ""
		}, timeout, interval).Should(BeTrue())
		Expect(repoAfter.Name).To(Equal(imageName))
		Expect(repoAfter.Namespace).To(Equal("default"))
		Expect(repoAfter.Status.CanonicalImageName).To(Equal("index.docker.io/library/alpine"))
	})

	It("fetches the tags for an image", func() {
		versions := []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
		imgRepo := loadImages(registryServer, "test-fetch", versions)

		repo = imagev1alpha1.ImageRepository{
			Spec: imagev1alpha1.ImageRepositorySpec{
				Image: imgRepo,
			},
		}
		objectName := types.NamespacedName{
			Name:      "random",
			Namespace: "default",
		}

		repo.Name = objectName.Name
		repo.Namespace = objectName.Namespace

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		r := imageRepoReconciler
		Expect(r.Create(ctx, &repo)).To(Succeed())

		var repoAfter imagev1alpha1.ImageRepository
		Eventually(func() bool {
			err := r.Get(context.Background(), objectName, &repoAfter)
			return err == nil && repoAfter.Status.CanonicalImageName != ""
		}, timeout, interval).Should(BeTrue())
		Expect(repoAfter.Status.CanonicalImageName).To(Equal(imgRepo))
		Expect(repoAfter.Status.LastScanResult.TagCount).To(Equal(len(versions)))
	})

	Context("when the ImageRepository is suspended", func() {
		It("does not process the image", func() {
			repo = imagev1alpha1.ImageRepository{
				Spec: imagev1alpha1.ImageRepositorySpec{
					Image:   "alpine",
					Suspend: true,
				},
			}
			imageRepoName := types.NamespacedName{
				Name:      imageName,
				Namespace: "default",
			}

			repo.Name = imageRepoName.Name
			repo.Namespace = imageRepoName.Namespace

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			r := imageRepoReconciler

			err := r.Create(ctx, &repo)
			Expect(err).ToNot(HaveOccurred())

			var repoAfter imagev1alpha1.ImageRepository
			Eventually(func() bool {
				err := r.Get(ctx, imageRepoName, &repoAfter)
				return err == nil && len(repoAfter.Status.Conditions) > 0
			}, timeout, interval).Should(BeTrue())
			Expect(repoAfter.Status.CanonicalImageName).To(Equal(""))
			cond := repoAfter.Status.Conditions[0]
			Expect(cond.Message).To(
				Equal("ImageRepository is suspended, skipping reconciliation"))
			Expect(cond.Reason).To(
				Equal(imagev1alpha1.SuspendedReason))
		})
	})

	Context("when the ImageRepository gets a 'reconcile at' annotation", func() {
		It("scans right away", func() {
			imgRepo := loadImages(registryServer, "test-fetch", []string{"1.0.0"})

			repo = imagev1alpha1.ImageRepository{
				Spec: imagev1alpha1.ImageRepositorySpec{
					Image: imgRepo,
				},
			}
			objectName := types.NamespacedName{
				Name:      "random",
				Namespace: "default",
			}

			repo.Name = objectName.Name
			repo.Namespace = objectName.Namespace

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			r := imageRepoReconciler
			err := r.Create(ctx, &repo)
			Expect(err).ToNot(HaveOccurred())

			// It'll get scanned on creation
			var repoAfter imagev1alpha1.ImageRepository
			Eventually(func() bool {
				err := r.Get(context.Background(), objectName, &repoAfter)
				return err == nil && repoAfter.Status.CanonicalImageName != ""
			}, timeout, interval).Should(BeTrue())

			lastScan := imagev1alpha1.GetLastTransitionTime(repoAfter)
			Expect(lastScan).ToNot(BeNil())

			requestToken := "this can be anything, so long as it's a change"

			repoAfter.Annotations = map[string]string{
				meta.ReconcileAtAnnotation: requestToken,
			}
			Expect(r.Update(ctx, &repoAfter)).To(Succeed())
			Eventually(func() bool {
				err := r.Get(context.Background(), objectName, &repoAfter)
				return err == nil && imagev1alpha1.GetLastTransitionTime(repoAfter).After(lastScan.Time)
			}, timeout, interval).Should(BeTrue())
			Expect(repoAfter.Status.LastHandledReconcileAt).To(Equal(requestToken))
		})
	})
})
