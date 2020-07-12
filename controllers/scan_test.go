/*
Copyright 2020 Michael Bridgen <mikeb@squaremobius.net>

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	imagev1alpha1 "github.com/squaremo/image-update/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("ImageRepository controller", func() {
	It("expands the canonical image name", func() {
		repo := imagev1alpha1.ImageRepository{
			Spec: imagev1alpha1.ImageRepositorySpec{
				Image: "alpine",
			},
		}
		imageRepoName := types.NamespacedName{
			Name:      "alpine-image",
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
			err = r.Get(context.Background(), imageRepoName, &repoAfter)
			return err == nil && repoAfter.Status.CanonicalImageName != ""
		}, timeout, interval).Should(BeTrue())
		Expect(repoAfter.Name).To(Equal("alpine-image"))
		Expect(repoAfter.Namespace).To(Equal("default"))
		Expect(repoAfter.Status.CanonicalImageName).To(Equal("index.docker.io/library/alpine"))
	})
})
