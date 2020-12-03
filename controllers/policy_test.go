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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	imagev1alpha1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// https://github.com/google/go-containerregistry/blob/v0.1.1/pkg/registry/compatibility_test.go
// has an example of loading a test registry with a random image.

var _ = Describe("ImagePolicy controller", func() {

	var registryServer *httptest.Server

	BeforeEach(func() {
		registryServer = newRegistryServer()
	})

	AfterEach(func() {
		registryServer.Close()
	})

	It("calculates an image from a repository's tags", func() {
		versions := []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
		imgRepo := loadImages(registryServer, "test-semver-policy", versions)

		repo := imagev1alpha1.ImageRepository{
			Spec: imagev1alpha1.ImageRepositorySpec{
				Interval: metav1.Duration{Duration: reconciliationInterval},
				Image:    imgRepo,
			},
		}
		imageObjectName := types.NamespacedName{
			Name:      "polimage",
			Namespace: "default",
		}
		repo.Name = imageObjectName.Name
		repo.Namespace = imageObjectName.Namespace

		ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
		defer cancel()

		r := imageRepoReconciler
		Expect(r.Create(ctx, &repo)).To(Succeed())

		var repoAfter imagev1alpha1.ImageRepository
		Eventually(func() bool {
			err := r.Get(ctx, imageObjectName, &repoAfter)
			return err == nil && repoAfter.Status.LastScanResult != nil
		}, timeout, interval).Should(BeTrue())
		Expect(repoAfter.Status.CanonicalImageName).To(Equal(imgRepo))
		Expect(repoAfter.Status.LastScanResult.TagCount).To(Equal(len(versions)))

		polName := types.NamespacedName{
			Name:      "random-pol",
			Namespace: imageObjectName.Namespace,
		}
		pol := imagev1alpha1.ImagePolicy{
			Spec: imagev1alpha1.ImagePolicySpec{
				ImageRepositoryRef: corev1.LocalObjectReference{
					Name: imageObjectName.Name,
				},
				Policy: imagev1alpha1.ImagePolicyChoice{
					SemVer: &imagev1alpha1.SemVerPolicy{
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

		var polAfter imagev1alpha1.ImagePolicy
		Eventually(func() bool {
			err := r.Get(ctx, polName, &polAfter)
			return err == nil && polAfter.Status.LatestImage != ""
		}, timeout, interval).Should(BeTrue())
		Expect(polAfter.Status.LatestImage).To(Equal(imgRepo + ":1.0.2"))
	})
})
