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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	conditionscheck "github.com/fluxcd/pkg/runtime/conditions/check"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/database"
	"github.com/fluxcd/image-reflector-controller/internal/test"
	// +kubebuilder:scaffold:imports
)

func TestImageRepositoryReconciler_canonicalImageName(t *testing.T) {
	g := NewWithT(t)

	// Would be good to test this without needing to do the scanning, since
	// 1. better to not rely on external services being available
	// 2. probably going to want to have several test cases
	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    "alpine",
		},
	}
	imageRepoName := types.NamespacedName{
		Name:      "test-canonical-name-" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = imageRepoName.Name
	repo.Namespace = imageRepoName.Namespace

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

	g.Eventually(func() bool {
		err := testEnv.Get(context.Background(), imageRepoName, &repo)
		return err == nil && repo.Status.LastScanResult != nil
	}, timeout).Should(BeTrue())
	g.Expect(repo.Name).To(Equal(imageRepoName.Name))
	g.Expect(repo.Namespace).To(Equal(imageRepoName.Namespace))
	g.Expect(repo.Status.CanonicalImageName).To(Equal("index.docker.io/library/alpine"))

	// Check if the object status is valid.
	condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
	checker := conditionscheck.NewChecker(testEnv.Client, condns)
	checker.WithT(g).CheckErr(ctx, &repo)

	// Cleanup.
	g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
}

func TestImageRepositoryReconciler_fetchImageTags(t *testing.T) {
	g := NewWithT(t)

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()
	tests := []struct {
		name          string
		versions      []string
		wantVersions  []string
		exclusionList []string
	}{
		{
			name:         "fetch image tags",
			versions:     []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.1.0", "1.1.0-alpha"},
			wantVersions: []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.1.0", "1.1.0-alpha"},
		},
		{
			name:         "fetch image tags - .sig is excluded",
			versions:     []string{"0.1.0", "0.1.1", "0.1.1.sig", "1.0.0-alpha", "1.0.0", "1.0.0.sig"},
			wantVersions: []string{"0.1.0", "0.1.1", "1.0.0-alpha", "1.0.0"},
		},
		{
			name:          "fetch image tags - tags in exclusionList are excluded",
			versions:      []string{"0.1.0", "0.1.1-alpha", "0.1.1", "0.1.1.sig", "1.0.0-alpha", "1.0.0"},
			wantVersions:  []string{"0.1.0", "0.1.1", "0.1.1.sig", "1.0.0"},
			exclusionList: []string{"^.*\\-alpha$"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imgRepo, _, err := test.LoadImages(registryServer, "test-fetch-"+randStringRunes(5), tt.versions)
			g.Expect(err).ToNot(HaveOccurred())

			repo := imagev1.ImageRepository{
				Spec: imagev1.ImageRepositorySpec{
					Interval:      metav1.Duration{Duration: reconciliationInterval},
					Image:         imgRepo,
					ExclusionList: tt.exclusionList,
				},
			}
			objectName := types.NamespacedName{
				Name:      "test-fetch-img-tags-" + randStringRunes(5),
				Namespace: "default",
			}

			repo.Name = objectName.Name
			repo.Namespace = objectName.Namespace

			ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
			defer cancel()
			g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

			g.Eventually(func() bool {
				err := testEnv.Get(context.Background(), objectName, &repo)
				return err == nil && repo.Status.LastScanResult != nil
			}, timeout, interval).Should(BeTrue())
			g.Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
			g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(tt.wantVersions)))
			g.Expect(repo.Status.LastScanResult.LatestTags).ToNot(BeEmpty())

			// Check if the object status is valid.
			condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
			checker := conditionscheck.NewChecker(testEnv.Client, condns)
			checker.WithT(g).CheckErr(ctx, &repo)

			// Cleanup.
			g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
		})
	}
}

func TestImageRepositoryReconciler_repositorySuspended(t *testing.T) {
	g := NewWithT(t)

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    "alpine",
			Suspend:  true,
		},
	}
	imageRepoName := types.NamespacedName{
		Name:      "test-suspended-repo-" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = imageRepoName.Name
	repo.Namespace = imageRepoName.Namespace

	// Add finalizer so that reconciliation reaches suspend check.
	controllerutil.AddFinalizer(&repo, imagev1.ImageFinalizer)

	builder := fakeclient.NewClientBuilder().WithScheme(testEnv.GetScheme())
	builder.WithObjects(&repo)

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	r := &ImageRepositoryReconciler{
		Client:       builder.Build(),
		Database:     database.NewBadgerDatabase(testBadgerDB),
		patchOptions: getPatchOptions(imageRepositoryOwnedConditions, "irc"),
	}

	key := client.ObjectKeyFromObject(&repo)
	res, err := r.Reconcile(context.TODO(), ctrl.Request{NamespacedName: key})
	g.Expect(err).To(BeNil())
	g.Expect(res.Requeue).ToNot(BeTrue())

	// Make sure no status was written.
	var ir imagev1.ImageRepository
	g.Expect(r.Get(ctx, imageRepoName, &ir)).ToNot(HaveOccurred())
	g.Expect(ir.Status.CanonicalImageName).To(Equal(""))
	// Cleanup.
	g.Expect(r.Delete(ctx, &ir)).To(Succeed())
}

func TestImageRepositoryReconciler_reconcileAtAnnotation(t *testing.T) {
	g := NewWithT(t)

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	imgRepo, _, err := test.LoadImages(registryServer, "test-annot-"+randStringRunes(5), []string{"1.0.0"})
	g.Expect(err).ToNot(HaveOccurred())

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    imgRepo,
		},
	}
	objectName := types.NamespacedName{
		Name:      "test-reconcile-at-annot-" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = objectName.Name
	repo.Namespace = objectName.Namespace

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()
	g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

	g.Eventually(func() bool {
		err := testEnv.Get(ctx, objectName, &repo)
		return err == nil && repo.Status.LastScanResult != nil
	}, timeout, interval).Should(BeTrue())

	requestToken := "this can be anything, so long as it's a change"
	lastScanTime := repo.Status.LastScanResult.ScanTime

	repo.Annotations = map[string]string{
		meta.ReconcileRequestAnnotation: requestToken,
	}
	g.Expect(testEnv.Update(ctx, &repo)).To(Succeed())
	g.Eventually(func() bool {
		err := testEnv.Get(ctx, objectName, &repo)
		return err == nil && repo.Status.LastScanResult.ScanTime.After(lastScanTime.Time)
	}, timeout, interval).Should(BeTrue())
	g.Expect(repo.Status.LastHandledReconcileAt).To(Equal(requestToken))

	// Check if the object status is valid.
	condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
	checker := conditionscheck.NewChecker(testEnv.Client, condns)
	checker.WithT(g).CheckErr(ctx, &repo)

	// Cleanup.
	g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
}

func TestImageRepositoryReconciler_authRegistry(t *testing.T) {
	g := NewWithT(t)

	username, password := "authuser", "authpass"
	registryServer := test.NewAuthenticatedRegistryServer(username, password)
	defer registryServer.Close()

	// this mimics what you get if you use
	//     kubectl create secret docker-registry ...
	secret := &corev1.Secret{
		Type: "kubernetes.io/dockerconfigjson",
		StringData: map[string]string{
			".dockerconfigjson": fmt.Sprintf(`
{
  "auths": {
    %q: {
      "username": %q,
      "password": %q
    }
  }
}
`, test.RegistryName(registryServer), username, password),
		},
	}
	secret.Namespace = "default"
	secret.Name = "docker"
	g.Expect(testEnv.Create(context.Background(), secret)).To(Succeed())
	defer func() {
		g.Expect(testEnv.Delete(context.Background(), secret)).To(Succeed())
	}()

	versions := []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
	imgRepo, _, err := test.LoadImages(registryServer, "test-authn-"+randStringRunes(5),
		versions, remote.WithAuth(&authn.Basic{
			Username: username,
			Password: password,
		}))
	g.Expect(err).ToNot(HaveOccurred())

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    imgRepo,
			SecretRef: &meta.LocalObjectReference{
				Name: "docker",
			},
		},
	}
	objectName := types.NamespacedName{
		Name:      "test-auth-reg-" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = objectName.Name
	repo.Namespace = objectName.Namespace

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()
	g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

	g.Eventually(func() bool {
		err := testEnv.Get(ctx, objectName, &repo)
		return err == nil && repo.Status.LastScanResult != nil
	}, timeout, interval).Should(BeTrue())
	g.Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
	g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

	// Check if the object status is valid.
	condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
	checker := conditionscheck.NewChecker(testEnv.Client, condns)
	checker.WithT(g).CheckErr(ctx, &repo)

	// Cleanup.
	g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
}

func TestImageRepositoryReconciler_imageAttribute_schemePrefix(t *testing.T) {
	g := NewWithT(t)

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	imgRepo, _, err := test.LoadImages(registryServer, "test-fetch", []string{"1.0.0"})
	g.Expect(err).ToNot(HaveOccurred())
	imgRepo = "https://" + imgRepo

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    imgRepo,
		},
	}
	objectName := types.NamespacedName{
		Name:      "schemeprefix" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = objectName.Name
	repo.Namespace = objectName.Namespace

	ctx, cancel := context.WithTimeout(context.TODO(), contextTimeout)
	defer cancel()
	g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

	var ready *metav1.Condition
	g.Eventually(func() bool {
		_ = testEnv.Get(ctx, objectName, &repo)
		ready = apimeta.FindStatusCondition(repo.GetConditions(), meta.ReadyCondition)
		return ready != nil && ready.Reason == imagev1.ImageURLInvalidReason
	}, timeout, interval).Should(BeTrue())
	g.Expect(ready.Message).To(ContainSubstring("should not include URL scheme"))

	// Check if the object status is valid.
	condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
	checker := conditionscheck.NewChecker(testEnv.Client, condns)
	checker.WithT(g).CheckErr(ctx, &repo)

	// Cleanup.
	g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
}

func TestImageRepositoryReconciler_imageAttribute_withTag(t *testing.T) {
	g := NewWithT(t)

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	imgRepo, _, err := test.LoadImages(registryServer, "test-fetch", []string{"1.0.0"})
	g.Expect(err).ToNot(HaveOccurred())
	imgRepo = imgRepo + ":1.0.0"

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    imgRepo,
		},
	}
	objectName := types.NamespacedName{
		Name:      "withtag" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = objectName.Name
	repo.Namespace = objectName.Namespace

	ctx, cancel := context.WithTimeout(context.TODO(), contextTimeout)
	defer cancel()
	g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

	var ready *metav1.Condition
	g.Eventually(func() bool {
		_ = testEnv.Get(ctx, objectName, &repo)
		ready = apimeta.FindStatusCondition(repo.GetConditions(), meta.ReadyCondition)
		return ready != nil && ready.Reason == imagev1.ImageURLInvalidReason
	}, timeout, interval).Should(BeTrue())
	g.Expect(ready.Message).To(ContainSubstring("should not contain a tag"))

	// Check if the object status is valid.
	condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
	checker := conditionscheck.NewChecker(testEnv.Client, condns)
	checker.WithT(g).CheckErr(ctx, &repo)

	// Cleanup.
	g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
}

func TestImageRepositoryReconciler_imageAttribute_hostPort(t *testing.T) {
	g := NewWithT(t)

	registryServer := test.NewRegistryServer()
	defer registryServer.Close()

	imgRepo, _, err := test.LoadImages(registryServer, "test-fetch", []string{"1.0.0"})
	g.Expect(err).ToNot(HaveOccurred())
	imgRepo = strings.ReplaceAll(imgRepo, "127.0.0.1", "localhost")

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval: metav1.Duration{Duration: reconciliationInterval},
			Image:    imgRepo,
		},
	}
	objectName := types.NamespacedName{
		Name:      "hostport" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = objectName.Name
	repo.Namespace = objectName.Namespace

	ctx, cancel := context.WithTimeout(context.TODO(), contextTimeout)
	defer cancel()
	g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

	g.Eventually(func() bool {
		err := testEnv.Get(ctx, objectName, &repo)
		return err == nil && repo.Status.LastScanResult != nil
	}, timeout, interval).Should(BeTrue())
	g.Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))

	// Check if the object status is valid.
	condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
	checker := conditionscheck.NewChecker(testEnv.Client, condns)
	checker.WithT(g).CheckErr(ctx, &repo)

	g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
}

func TestImageRepositoryReconciler_authRegistryWithServiceAccount(t *testing.T) {
	g := NewWithT(t)

	username, password := "authuser", "authpass"
	registryServer := test.NewAuthenticatedRegistryServer(username, password)
	defer registryServer.Close()

	// this mimics what you get if you use
	//     kubectl create secret docker-registry ...
	secret := &corev1.Secret{
		Type: "kubernetes.io/dockerconfigjson",
		StringData: map[string]string{
			".dockerconfigjson": fmt.Sprintf(`
{
  "auths": {
    %q: {
      "username": %q,
      "password": %q
    }
  }
}
`, test.RegistryName(registryServer), username, password),
		},
	}
	secret.Namespace = "default"
	secret.Name = "docker"

	serviceAccount := &corev1.ServiceAccount{
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "docker"}},
	}
	serviceAccount.Name = "test-sa"
	serviceAccount.Namespace = "default"
	g.Expect(testEnv.Create(context.Background(), secret)).To(Succeed())
	g.Expect(testEnv.Create(context.Background(), serviceAccount)).To(Succeed())
	defer func() {
		g.Expect(testEnv.Delete(context.Background(), secret)).To(Succeed())
		g.Expect(testEnv.Delete(context.Background(), serviceAccount)).To(Succeed())
	}()

	versions := []string{"0.1.0", "0.1.1", "0.2.0", "1.0.0", "1.0.1", "1.0.2", "1.1.0-alpha"}
	imgRepo, _, err := test.LoadImages(registryServer, "test-authn-"+randStringRunes(5),
		versions, remote.WithAuth(&authn.Basic{
			Username: username,
			Password: password,
		}))
	g.Expect(err).ToNot(HaveOccurred())

	repo := imagev1.ImageRepository{
		Spec: imagev1.ImageRepositorySpec{
			Interval:           metav1.Duration{Duration: reconciliationInterval},
			Image:              imgRepo,
			ServiceAccountName: "test-sa",
		},
	}
	objectName := types.NamespacedName{
		Name:      "test-auth-reg-" + randStringRunes(5),
		Namespace: "default",
	}

	repo.Name = objectName.Name
	repo.Namespace = objectName.Namespace

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()
	g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

	g.Eventually(func() bool {
		err := testEnv.Get(ctx, objectName, &repo)
		return err == nil && repo.Status.LastScanResult != nil
	}, timeout, interval).Should(BeTrue())
	g.Expect(repo.Status.CanonicalImageName).To(Equal(imgRepo))
	g.Expect(repo.Status.LastScanResult.TagCount).To(Equal(len(versions)))

	// Check if the object status is valid.
	condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
	checker := conditionscheck.NewChecker(testEnv.Client, condns)
	checker.WithT(g).CheckErr(ctx, &repo)

	// Cleanup.
	g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
}

func TestImageRepositoryReconciler_ScanPublicRepos(t *testing.T) {
	tests := []struct {
		name  string
		image string
	}{
		{"k8s", "registry.k8s.io/coredns/coredns"},
		{"ghcr", "ghcr.io/stefanprodan/podinfo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			repo := imagev1.ImageRepository{
				Spec: imagev1.ImageRepositorySpec{
					Interval: metav1.Duration{Duration: time.Hour},
					Image:    tt.image,
				},
			}
			objectName := types.NamespacedName{
				Name:      "public-repo" + randStringRunes(5),
				Namespace: "default",
			}
			repo.Name = objectName.Name
			repo.Namespace = objectName.Namespace

			ctx, cancel := context.WithTimeout(context.TODO(), contextTimeout)
			defer cancel()
			g.Expect(testEnv.Create(ctx, &repo)).To(Succeed())

			g.Eventually(func() bool {
				err := testEnv.Get(ctx, objectName, &repo)
				return err == nil && repo.Status.LastScanResult != nil
			}, timeout, interval).Should(BeTrue())
			g.Expect(repo.Status.LastScanResult.TagCount).ToNot(BeZero())

			// Check if the object status is valid.
			condns := &conditionscheck.Conditions{NegativePolarity: imageRepositoryNegativeConditions}
			checker := conditionscheck.NewChecker(testEnv.Client, condns)
			checker.WithT(g).CheckErr(ctx, &repo)

			g.Expect(testEnv.Delete(ctx, &repo)).To(Succeed())
		})
	}
}
