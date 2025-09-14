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

package integration

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/fluxcd/pkg/apis/meta"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
)

func TestImageRepositoryScanAndPolicyDigest(t *testing.T) {
	for name, repo := range testRepos {
		t.Run(name, func(t *testing.T) {
			testImageRepositoryScanAndPolicyDigest(t, repo)
		})
	}
}

func testImageRepositoryScanAndPolicyDigest(t *testing.T, repoURL string) {
	g := NewWithT(t)
	ctx := context.TODO()

	repo := &imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: v1.Duration{Duration: 30 * time.Second},
			Image:    repoURL,
			Provider: *targetProvider,
		},
	}
	repoObjectKey := types.NamespacedName{
		Name:      "test-repo-" + randStringRunes(5),
		Namespace: "default",
	}
	repo.Name = repoObjectKey.Name
	repo.Namespace = repoObjectKey.Namespace

	g.Expect(testEnv.Client.Create(ctx, repo)).To(Succeed())
	defer func() {
		g.Expect(testEnv.Client.Delete(ctx, repo)).To(Succeed())
	}()
	g.Eventually(func() bool {
		if err := testEnv.Client.Get(ctx, repoObjectKey, repo); err != nil {
			return false
		}
		return repo.Status.LastScanResult != nil
	}, resultWaitTimeout).Should(BeTrue())
	g.Expect(repo.Status.CanonicalImageName).To(Equal(repoURL))
	g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(4))

	policy := &imagev1.ImagePolicy{
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: repo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				Alphabetical: &imagev1.AlphabeticalPolicy{},
			},
			DigestReflectionPolicy: imagev1.ReflectIfNotPresent,
		},
	}
	policyObjectKey := types.NamespacedName{
		Name:      "test-policy-" + randStringRunes(5),
		Namespace: "default",
	}
	policy.Name = policyObjectKey.Name
	policy.Namespace = policyObjectKey.Namespace

	g.Expect(testEnv.Client.Create(ctx, policy)).To(Succeed())
	defer func() {
		g.Expect(testEnv.Client.Delete(ctx, policy)).To(Succeed())
	}()
	g.Eventually(func() bool {
		if err := testEnv.Client.Get(ctx, policyObjectKey, policy); err != nil {
			return false
		}
		return policy.Status.LatestRef != nil && policy.Status.LatestRef.Digest != ""
	}, resultWaitTimeout).Should(BeTrue())
	g.Expect(policy.Status.LatestRef.Digest).To(HavePrefix("sha256:"))
	g.Expect(policy.Status.LatestRef.Digest).To(HaveLen(71))
}
