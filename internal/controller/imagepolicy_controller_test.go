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
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aclapis "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
	"github.com/fluxcd/image-reflector-controller/internal/policy"
	"github.com/fluxcd/image-reflector-controller/internal/registry"
	"github.com/fluxcd/image-reflector-controller/internal/test"
)

func TestImagePolicyReconciler_imageRepoHasNoTags(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imageRepo := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "repo",
		},
		Spec: imagev1.ImageRepositorySpec{
			Image: "ghcr.io/doesnot/exist",
		},
	}
	g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
	})

	imagePolicy := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				Alphabetical: &imagev1.AlphabeticalPolicy{},
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && !conditions.IsReady(imagePolicy) &&
			conditions.GetReason(imagePolicy, meta.ReadyCondition) == imagev1.DependencyNotReadyReason
	}).Should(BeTrue())
}

func TestImagePolicyReconciler_ignoresImageRepoNotReadyEvent(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imageRepo := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "repo",
		},
		Spec: imagev1.ImageRepositorySpec{
			Image: "ghcr.io/stefanprodan/podinfo",
		},
	}
	g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		return err == nil && conditions.IsReady(imageRepo)
	}, timeout).Should(BeTrue())

	imagePolicy := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				Alphabetical: &imagev1.AlphabeticalPolicy{},
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy)
	}).Should(BeTrue())

	// Now cause the ImageRepository to become not ready.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		if err != nil {
			return false
		}
		p := patch.NewSerialPatcher(imageRepo, k8sClient)
		imageRepo.Spec.Image = "ghcr.io/stefanprodan/podinfo/foo:bar:zzz:qqq/aaa"
		return p.Patch(ctx, imageRepo) == nil
	}).Should(BeTrue())

	// Wait for the ImageRepository to become not ready.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		return err == nil && conditions.IsStalled(imageRepo)
	}).Should(BeTrue())

	// Check that the ImagePolicy is still ready and does not get updated.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy)
	}, timeout, interval).Should(BeTrue())

	// Wait a bit and check that the ImagePolicy remains ready.
	time.Sleep(time.Second)
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy)
	}, timeout, interval).Should(BeTrue())
}

func TestImagePolicyReconciler_imageRepoRevisionLifeCycle(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imageRepo := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "repo",
		},
		Spec: imagev1.ImageRepositorySpec{
			Image: "ghcr.io/stefanprodan/podinfo",
		},
	}
	g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		return err == nil && conditions.IsReady(imageRepo) &&
			imageRepo.Generation == conditions.GetObservedGeneration(imageRepo, meta.ReadyCondition)
	}, timeout).Should(BeTrue())

	imagePolicy := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
			FilterTags: &imagev1.TagFilter{
				Pattern: `^6\.7\.\d+$`,
			},
			Policy: imagev1.ImagePolicyChoice{
				SemVer: &imagev1.SemVerPolicy{
					Range: "6.7.x",
				},
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy) &&
			imagePolicy.Generation == conditions.GetObservedGeneration(imagePolicy, meta.ReadyCondition) &&
			imagePolicy.Status.LatestRef != nil &&
			imagePolicy.Status.LatestRef.Tag == "6.7.1"
	}, timeout).Should(BeTrue())
	expectedImagePolicyLastTransitionTime := conditions.GetLastTransitionTime(imagePolicy, meta.ReadyCondition).Time

	// Now force a reconciliation by setting the annotation.
	var requestedAt string
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		if err != nil {
			return false
		}
		p := patch.NewSerialPatcher(imageRepo, k8sClient)
		requestedAt = time.Now().Format(time.RFC3339Nano)
		if imageRepo.Annotations == nil {
			imageRepo.Annotations = make(map[string]string)
		}
		imageRepo.Annotations["reconcile.fluxcd.io/requestedAt"] = requestedAt
		return p.Patch(ctx, imageRepo) == nil
	}, timeout).Should(BeTrue())

	// Wait for the ImageRepository to reconcile.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		return err == nil && conditions.IsReady(imageRepo) &&
			imageRepo.Status.LastHandledReconcileAt == requestedAt
	}, timeout).Should(BeTrue())

	// Check that the ImagePolicy is still ready and does not get updated.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy) &&
			imagePolicy.Status.LatestRef != nil &&
			imagePolicy.Status.LatestRef.Tag == "6.7.1"
	}, timeout).Should(BeTrue())

	// Wait a bit and check that the ImagePolicy remains ready.
	time.Sleep(time.Second)
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy) &&
			imagePolicy.Status.LatestRef != nil &&
			imagePolicy.Status.LatestRef.Tag == "6.7.1"
	}, timeout).Should(BeTrue())

	// Check that the last transition time of the ImagePolicy Ready condition did not change since the beginning.
	lastTransitionTime := conditions.GetLastTransitionTime(imagePolicy, meta.ReadyCondition).Time
	g.Expect(lastTransitionTime).To(Equal(expectedImagePolicyLastTransitionTime))

	// Now add an exclusion rule to force the checksum to change.
	firstChecksum := imageRepo.Status.LastScanResult.Revision
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		if err != nil {
			return false
		}
		p := patch.NewSerialPatcher(imageRepo, k8sClient)
		imageRepo.Spec.ExclusionList = []string{`^6\.7\.1$`}
		return p.Patch(ctx, imageRepo) == nil
	}, timeout).Should(BeTrue())

	// Wait for the ImageRepository to reconcile.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		return err == nil && conditions.IsReady(imageRepo) &&
			imageRepo.Generation == conditions.GetObservedGeneration(imageRepo, meta.ReadyCondition) &&
			imageRepo.Status.LastScanResult.Revision != firstChecksum
	}, timeout).Should(BeTrue())

	// Check that the ImagePolicy receives the update and the latest tag changes.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		if err != nil {
			return false
		}
		return conditions.IsReady(imagePolicy) &&
			imagePolicy.Generation == conditions.GetObservedGeneration(imagePolicy, meta.ReadyCondition) &&
			imagePolicy.Status.LatestRef.Tag == "6.7.0"
	}, timeout).Should(BeTrue())
}

func TestImagePolicyReconciler_invalidImage(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imageRepo := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "repo",
		},
		Spec: imagev1.ImageRepositorySpec{
			Image:   "ghcr.io/stefanprodan/podinfo/foo:bar:zzz:qqq/aaa",
			Suspend: true,
		},
	}
	g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		if err != nil {
			return false
		}
		p := patch.NewSerialPatcher(imageRepo, k8sClient)
		conditions.MarkTrue(imageRepo, meta.ReadyCondition, "success", "image repository is ready")
		return p.Patch(ctx, imageRepo) == nil
	}).Should(BeTrue())

	imagePolicy := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsStalled(imagePolicy) &&
			conditions.GetReason(imagePolicy, meta.StalledCondition) == imagev1.ImageURLInvalidReason
	}).Should(BeTrue())
}

func TestImagePolicyReconciler_objectLevelWorkloadIdentityFeatureGate(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		g := NewWithT(t)

		namespaceName := "imagepolicy-" + randStringRunes(5)
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
		}
		g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
		t.Cleanup(func() {
			g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
		})

		imageRepo := &imagev1.ImageRepository{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
				Name:      "repo",
			},
			Spec: imagev1.ImageRepositorySpec{
				Image:              "ghcr.io/stefanprodan/podinfo",
				Provider:           "aws",
				ServiceAccountName: "foo",
			},
		}
		g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
		t.Cleanup(func() {
			g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
		})

		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
			return err == nil && conditions.IsStalled(imageRepo) &&
				conditions.GetReason(imageRepo, meta.StalledCondition) == meta.FeatureGateDisabledReason &&
				conditions.GetMessage(imageRepo, meta.StalledCondition) == "to use spec.serviceAccountName for provider authentication please enable the ObjectLevelWorkloadIdentity feature gate in the controller"
		}).Should(BeTrue())

		g.Eventually(func() bool {
			p := patch.NewSerialPatcher(imageRepo, k8sClient)
			imageRepo.Spec.Suspend = true
			imageRepo.Status.Conditions = nil
			conditions.MarkTrue(imageRepo, meta.ReadyCondition, "success", "image repository is ready")
			return p.Patch(ctx, imageRepo) == nil
		}).Should(BeTrue())

		imagePolicy := &imagev1.ImagePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
				Name:      "test-imagepolicy",
			},
			Spec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name: imageRepo.Name,
				},
				Policy: imagev1.ImagePolicyChoice{
					Alphabetical: &imagev1.AlphabeticalPolicy{},
				},
			},
		}
		g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
		t.Cleanup(func() {
			g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
		})

		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
			logPolicyStatus(t, imagePolicy)
			return err == nil && conditions.IsStalled(imagePolicy) &&
				conditions.GetReason(imagePolicy, meta.StalledCondition) == meta.FeatureGateDisabledReason &&
				conditions.GetMessage(imagePolicy, meta.StalledCondition) == "to use spec.serviceAccountName in the ImageRepository for provider authentication please enable the ObjectLevelWorkloadIdentity feature gate in the controller"
		}).Should(BeTrue())
	})

	t.Run("enabled", func(t *testing.T) {
		g := NewWithT(t)

		auth.EnableObjectLevelWorkloadIdentity()
		t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)

		namespaceName := "imagepolicy-" + randStringRunes(5)
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
		}
		g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
		t.Cleanup(func() {
			g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
		})

		imageRepo := &imagev1.ImageRepository{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
				Name:      "repo",
			},
			Spec: imagev1.ImageRepositorySpec{
				Image:              "ghcr.io/stefanprodan/podinfo",
				Provider:           "aws",
				ServiceAccountName: "foo",
			},
		}
		g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
		t.Cleanup(func() {
			g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
		})

		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
			logRepoStatus(t, imageRepo)
			return err == nil && !conditions.IsReady(imageRepo) &&
				conditions.GetReason(imageRepo, meta.ReadyCondition) == imagev1.AuthenticationFailedReason
		}).Should(BeTrue())

		g.Eventually(func() bool {
			p := patch.NewSerialPatcher(imageRepo, k8sClient)
			imageRepo.Spec.Suspend = true
			imageRepo.Status.Conditions = nil
			conditions.MarkTrue(imageRepo, meta.ReadyCondition, "success", "image repository is ready")
			return p.Patch(ctx, imageRepo) == nil
		}).Should(BeTrue())

		imagePolicy := &imagev1.ImagePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespaceName,
				Name:      "test-imagepolicy",
			},
			Spec: imagev1.ImagePolicySpec{
				ImageRepositoryRef: meta.NamespacedObjectReference{
					Name: imageRepo.Name,
				},
				Policy: imagev1.ImagePolicyChoice{
					Alphabetical: &imagev1.AlphabeticalPolicy{},
				},
			},
		}
		g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
		t.Cleanup(func() {
			g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
		})

		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
			logPolicyStatus(t, imagePolicy)
			return err == nil && !conditions.IsReady(imagePolicy) &&
				conditions.GetReason(imagePolicy, meta.ReadyCondition) == imagev1.DependencyNotReadyReason
		}).Should(BeTrue())
	})
}

func TestImagePolicyReconciler_intervalNotConfigured(t *testing.T) {
	g := NewWithT(t)

	r := &ImagePolicyReconciler{
		Client:        k8sClient,
		EventRecorder: record.NewFakeRecorder(32),
	}

	obj := &imagev1.ImagePolicy{
		Spec: imagev1.ImagePolicySpec{
			DigestReflectionPolicy: imagev1.ReflectAlways,
		},
	}

	res, err := r.reconcile(ctx, nil, obj)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).To(Equal(ctrl.Result{}))

	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.IsReady(obj)).To(BeFalse())
	g.Expect(conditions.GetReason(obj, meta.StalledCondition)).To(Equal(imagev1.IntervalNotConfiguredReason))
	g.Expect(conditions.GetReason(obj, meta.ReadyCondition)).To(Equal(imagev1.IntervalNotConfiguredReason))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(Equal("spec.interval must be set when spec.digestReflectionPolicy is set to 'Always'"))
	g.Expect(conditions.GetMessage(obj, meta.ReadyCondition)).To(Equal("spec.interval must be set when spec.digestReflectionPolicy is set to 'Always'"))
}

func TestImagePolicyReconciler_apiServerValidation(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	// Test invalid ImagePolicy spec due to interval set without digestReflectionPolicy=Always.
	imagePolicy := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: "foo",
			},
			Policy:   imagev1.ImagePolicyChoice{},
			Interval: &metav1.Duration{Duration: time.Minute},
		},
	}
	err := k8sClient.Create(ctx, imagePolicy)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.interval is only accepted when spec.digestReflectionPolicy is set to 'Always'"))

	// Test invalid ImagePolicy spec due to digestReflectionPolicy=Always set without interval.
	imagePolicy.Spec.DigestReflectionPolicy = imagev1.ReflectAlways
	imagePolicy.Spec.Interval = nil
	err = k8sClient.Create(ctx, imagePolicy)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.interval must be set when spec.digestReflectionPolicy is set to 'Always'"))

	// Test creating valid ImagePolicy spec.
	imagePolicy.Spec.Interval = &metav1.Duration{Duration: time.Minute}
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
}

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
		requeueAfter1stPass time.Duration
		requeueAfter2ndPass time.Duration
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
			requeueAfter1stPass: 10 * time.Minute,
			requeueAfter2ndPass: 10 * time.Minute,
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
			requeueAfter1stPass: 10 * time.Minute,
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
			requeueAfter2ndPass: 10 * time.Minute,
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
			requeueAfter1stPass: 10 * time.Minute,
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
					Conditions: []metav1.Condition{
						{
							Type:   meta.ReadyCondition,
							Status: metav1.ConditionTrue,
						},
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
					Interval: &metav1.Duration{Duration: 10 * time.Minute},
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
				EventRecorder:     record.NewFakeRecorder(32),
				Client:            c,
				Database:          &mockDatabase{TagData: imageRepo.Status.LastScanResult.LatestTags},
				AuthOptionsGetter: &registry.AuthOptionsGetter{Client: c},
			}

			res, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: ns.Name,
					Name:      imagePol.Name,
				},
			})

			g.Expect(err).NotTo(HaveOccurred(), "reconciliation failed")
			g.Expect(res).To(Equal(ctrl.Result{RequeueAfter: tt.requeueAfter1stPass}))

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
			g.Expect(res).To(Equal(ctrl.Result{RequeueAfter: tt.requeueAfter2ndPass}))

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

			result, err := r.applyPolicy(obj, repo)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if err == nil {
				g.Expect(result).To(Equal(tt.wantResult))
			}
		})
	}
}

func TestComposeImagePolicyReadyMessage(t *testing.T) {
	tests := []struct {
		name        string
		obj         *imagev1.ImagePolicy
		wantMessage string
	}{
		{
			name: "no previous tag",
			obj: &imagev1.ImagePolicy{
				Status: imagev1.ImagePolicyStatus{
					LatestRef: &imagev1.ImageRef{
						Name: "foo/bar",
						Tag:  "1.0.0",
					},
				},
			},
			wantMessage: "Latest image tag for foo/bar resolved to 1.0.0",
		},
		{
			name: "different previous tag",
			obj: &imagev1.ImagePolicy{
				Status: imagev1.ImagePolicyStatus{
					LatestRef: &imagev1.ImageRef{
						Name: "foo/bar",
						Tag:  "1.1.0",
					},
					ObservedPreviousRef: &imagev1.ImageRef{
						Name: "foo/bar",
						Tag:  "1.0.0",
					},
				},
			},
			wantMessage: "Latest image tag for foo/bar resolved to 1.1.0 (previously foo/bar:1.0.0)",
		},
		{
			name: "same previous and latest tags",
			obj: &imagev1.ImagePolicy{
				Status: imagev1.ImagePolicyStatus{
					LatestRef: &imagev1.ImageRef{
						Name: "foo/bar",
						Tag:  "1.0.0",
					},
					ObservedPreviousRef: &imagev1.ImageRef{
						Name: "foo/bar",
						Tag:  "1.0.0",
					},
				},
			},
			wantMessage: "Latest image tag for foo/bar resolved to 1.0.0",
		},
		{
			name: "different image with digest",
			obj: &imagev1.ImagePolicy{
				Status: imagev1.ImagePolicyStatus{
					LatestRef: &imagev1.ImageRef{
						Name:   "foo/bar",
						Tag:    "1.0.0",
						Digest: "sha256:1234567890abcdef",
					},
					ObservedPreviousRef: &imagev1.ImageRef{
						Name: "baz/qux",
						Tag:  "1.0.0",
					},
				},
			},
			wantMessage: "Latest image tag for foo/bar resolved to 1.0.0 with digest sha256:1234567890abcdef (previously baz/qux:1.0.0)",
		},
		{
			name: "different digest",
			obj: &imagev1.ImagePolicy{
				Status: imagev1.ImagePolicyStatus{
					LatestRef: &imagev1.ImageRef{
						Name:   "foo/bar",
						Tag:    "1.0.0",
						Digest: "sha256:1234567890abcdef",
					},
					ObservedPreviousRef: &imagev1.ImageRef{
						Name:   "foo/bar",
						Tag:    "1.0.0",
						Digest: "sha256:abcdef1234567890",
					},
				},
			},
			wantMessage: "Latest image tag for foo/bar resolved to 1.0.0 with digest sha256:1234567890abcdef (previously foo/bar:1.0.0@sha256:abcdef1234567890)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := composeImagePolicyReadyMessage(tt.obj)
			g.Expect(result).To(Equal(tt.wantMessage))
		})
	}
}

func TestImagePolicyReconciler_intervalBasedReconciliation(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imageRepo := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "repo",
		},
		Spec: imagev1.ImageRepositorySpec{
			Image: "ghcr.io/stefanprodan/podinfo",
		},
	}
	g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
			return apierrors.IsNotFound(err)
		}, timeout).Should(BeTrue())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		return err == nil && conditions.IsReady(imageRepo)
	}, timeout).Should(BeTrue())

	// Create ImagePolicy with DigestReflectionPolicy=Always and 1-minute interval
	imagePolicy := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				Alphabetical: &imagev1.AlphabeticalPolicy{},
			},
			DigestReflectionPolicy: imagev1.ReflectAlways,
			Interval:               &metav1.Duration{Duration: time.Minute},
		},
	}
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
			return apierrors.IsNotFound(err)
		}, timeout).Should(BeTrue())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy) &&
			imagePolicy.Status.LatestRef != nil &&
			imagePolicy.Status.LatestRef.Digest != "" // Should have digest when Always policy
	}, timeout).Should(BeTrue())

	g.Expect(imagePolicy.Status.LatestRef.Digest).ToNot(BeEmpty())

	// Create another ImagePolicy without interval (DigestReflectionPolicy=Never by default)
	imagePolicyNoInterval := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy-no-interval",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				Alphabetical: &imagev1.AlphabeticalPolicy{},
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, imagePolicyNoInterval)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imagePolicyNoInterval)).NotTo(HaveOccurred())
		g.Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicyNoInterval), imagePolicyNoInterval)
			return apierrors.IsNotFound(err)
		}, timeout).Should(BeTrue())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicyNoInterval), imagePolicyNoInterval)
		return err == nil && conditions.IsReady(imagePolicyNoInterval) &&
			imagePolicyNoInterval.Status.LatestRef != nil
	}, timeout).Should(BeTrue())

	g.Expect(imagePolicyNoInterval.Status.LatestRef.Digest).To(BeEmpty())
}

func TestImagePolicyReconciler_suspend(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	obj := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			Suspend: true,
		},
	}

	g.Expect(k8sClient.Create(ctx, obj)).NotTo(HaveOccurred())

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		return err == nil &&
			controllerutil.ContainsFinalizer(obj, imagev1.ImageFinalizer)
	}, timeout).Should(BeTrue())

	// Wait a bit and observe that the policy status did not change.
	time.Sleep(time.Second)

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		return err == nil && obj.Status.ObservedGeneration == -1
	}, timeout).Should(BeTrue())
}

func TestImagePolicyReconciler_reconcileRequestStatus(t *testing.T) {
	g := NewWithT(t)

	namespaceName := "imagepolicy-" + randStringRunes(5)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
	}
	g.Expect(k8sClient.Create(ctx, namespace)).ToNot(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, namespace)).NotTo(HaveOccurred())
	})

	imageRepo := &imagev1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "repo",
		},
		Spec: imagev1.ImageRepositorySpec{
			Image: "ghcr.io/stefanprodan/podinfo",
		},
	}
	g.Expect(k8sClient.Create(ctx, imageRepo)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imageRepo)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imageRepo), imageRepo)
		return err == nil && conditions.IsReady(imageRepo)
	}, timeout).Should(BeTrue())

	imagePolicy := &imagev1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      "test-imagepolicy",
		},
		Spec: imagev1.ImagePolicySpec{
			ImageRepositoryRef: meta.NamespacedObjectReference{
				Name: imageRepo.Name,
			},
			Policy: imagev1.ImagePolicyChoice{
				Alphabetical: &imagev1.AlphabeticalPolicy{},
			},
		},
	}
	g.Expect(k8sClient.Create(ctx, imagePolicy)).NotTo(HaveOccurred())
	t.Cleanup(func() {
		g.Expect(k8sClient.Delete(ctx, imagePolicy)).NotTo(HaveOccurred())
	})

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil && conditions.IsReady(imagePolicy)
	}, timeout).Should(BeTrue())

	// Patch the annotation to simulate a reconcile request.
	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		if err != nil {
			return false
		}
		p := patch.NewSerialPatcher(imagePolicy, k8sClient)
		if imagePolicy.Annotations == nil {
			imagePolicy.Annotations = map[string]string{}
		}
		imagePolicy.Annotations["reconcile.fluxcd.io/requestedAt"] = "some-string"
		return p.Patch(ctx, imagePolicy) == nil
	}, timeout).Should(BeTrue())

	g.Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(imagePolicy), imagePolicy)
		return err == nil &&
			conditions.IsReady(imagePolicy) &&
			imagePolicy.Status.LastHandledReconcileAt == "some-string"
	}, timeout).Should(BeTrue())
}
