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

package controllers

import (
	"context"
	"errors"
	"testing"

	aclapis "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/acl"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/policy"
)

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
