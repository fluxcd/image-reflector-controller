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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImagePolicySpec defines the parameters for calculating the
// ImagePolicy
type ImagePolicySpec struct {
	// ImageRepository points at the object specifying the image being
	// scanned
	// +required
	ImageRepository corev1.LocalObjectReference `json:"imageRepository"`
	// Policy gives the particulars of the policy to be followed in
	// selecting the most recent image
	// +required
	Policy ImagePolicyChoice `json:"policy"`
}

// ImagePolicyChoice is a union of all the types of policy that can be
// supplied.
type ImagePolicyChoice struct {
	// SemVer gives a semantic version range to check against the tags
	// available.
	// +optional
	SemVer *SemVerPolicy `json:"semver,omitempty"`
}

// SemVerPolicy specifices a semantic version policy.
type SemVerPolicy struct {
	// Range gives a semver range for the image tag; the highest
	// version within the range that's a tag yields the latest image.
	// +required
	Range string `json:"range"`
}

// ImagePolicyStatus defines the observed state of ImagePolicy
type ImagePolicyStatus struct {
	// LatestImage gives the first in the list of images scanned by
	// the image repository, when filtered and ordered according to
	// the policy.
	LatestImage string `json:"latestImage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="LatestImage",type=string,JSONPath=`.status.latestImage`

// ImagePolicy is the Schema for the imagepolicies API
type ImagePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImagePolicySpec   `json:"spec,omitempty"`
	Status ImagePolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ImagePolicyList contains a list of ImagePolicy
type ImagePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImagePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ImagePolicy{}, &ImagePolicyList{})
}
