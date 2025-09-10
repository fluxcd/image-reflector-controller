/*
Copyright 2023 The Flux authors

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

package v1beta2

import (
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ImagePolicyKind = "ImagePolicy"

// Deprecated: Use ImageFinalizer.
const ImagePolicyFinalizer = ImageFinalizer

// ImagePolicySpec defines the parameters for calculating the
// ImagePolicy.
// +kubebuilder:validation:XValidation:rule="!has(self.interval) || (has(self.digestReflectionPolicy) && self.digestReflectionPolicy == 'Always')", message="spec.interval is only accepted when spec.digestReflectionPolicy is set to 'Always'"
// +kubebuilder:validation:XValidation:rule="has(self.interval) || !has(self.digestReflectionPolicy) || self.digestReflectionPolicy != 'Always'", message="spec.interval must be set when spec.digestReflectionPolicy is set to 'Always'"
type ImagePolicySpec struct {
	// ImageRepositoryRef points at the object specifying the image
	// being scanned
	// +required
	ImageRepositoryRef meta.NamespacedObjectReference `json:"imageRepositoryRef"`
	// Policy gives the particulars of the policy to be followed in
	// selecting the most recent image
	// +required
	Policy ImagePolicyChoice `json:"policy"`
	// FilterTags enables filtering for only a subset of tags based on a set of
	// rules. If no rules are provided, all the tags from the repository will be
	// ordered and compared.
	// +optional
	FilterTags *TagFilter `json:"filterTags,omitempty"`
	// DigestReflectionPolicy governs the setting of the `.status.latestRef.digest` field.
	//
	// Never: The digest field will always be set to the empty string.
	//
	// IfNotPresent: The digest field will be set to the digest of the elected
	// latest image if the field is empty and the image did not change.
	//
	// Always: The digest field will always be set to the digest of the elected
	// latest image.
	//
	// Default: Never.
	// +kubebuilder:default:=Never
	DigestReflectionPolicy ReflectionPolicy `json:"digestReflectionPolicy,omitempty"`

	// Interval is the length of time to wait between
	// refreshing the digest of the latest tag when the
	// reflection policy is set to "Always".
	//
	// Defaults to 10m.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// This flag tells the controller to suspend subsequent policy reconciliations.
	// It does not apply to already started reconciliations. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ReflectionPolicy describes a policy for if/when to reflect a value from the registry in a certain resource field.
// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
type ReflectionPolicy string

const (
	// ReflectAlways means that a value is always reflected with the latest value from the registry even if this would
	// overwrite an existing value in the object.
	ReflectAlways ReflectionPolicy = "Always"
	// ReflectIfNotPresent means that the target value is only reflected from the registry if it is empty. It will
	// never be overwritten afterwards, even if it changes in the registry.
	ReflectIfNotPresent ReflectionPolicy = "IfNotPresent"
	// ReflectNever means that no reflection will happen at all.
	ReflectNever ReflectionPolicy = "Never"
)

// ImagePolicyChoice is a union of all the types of policy that can be
// supplied.
type ImagePolicyChoice struct {
	// SemVer gives a semantic version range to check against the tags
	// available.
	// +optional
	SemVer *SemVerPolicy `json:"semver,omitempty"`
	// Alphabetical set of rules to use for alphabetical ordering of the tags.
	// +optional
	Alphabetical *AlphabeticalPolicy `json:"alphabetical,omitempty"`
	// Numerical set of rules to use for numerical ordering of the tags.
	// +optional
	Numerical *NumericalPolicy `json:"numerical,omitempty"`
}

// SemVerPolicy specifies a semantic version policy.
type SemVerPolicy struct {
	// Range gives a semver range for the image tag; the highest
	// version within the range that's a tag yields the latest image.
	// +required
	Range string `json:"range"`
}

// AlphabeticalPolicy specifies a alphabetical ordering policy.
type AlphabeticalPolicy struct {
	// Order specifies the sorting order of the tags. Given the letters of the
	// alphabet as tags, ascending order would select Z, and descending order
	// would select A.
	// +kubebuilder:default:="asc"
	// +kubebuilder:validation:Enum=asc;desc
	// +optional
	Order string `json:"order,omitempty"`
}

// NumericalPolicy specifies a numerical ordering policy.
type NumericalPolicy struct {
	// Order specifies the sorting order of the tags. Given the integer values
	// from 0 to 9 as tags, ascending order would select 9, and descending order
	// would select 0.
	// +kubebuilder:default:="asc"
	// +kubebuilder:validation:Enum=asc;desc
	// +optional
	Order string `json:"order,omitempty"`
}

// TagFilter enables filtering tags based on a set of defined rules
type TagFilter struct {
	// Pattern specifies a regular expression pattern used to filter for image
	// tags.
	// +optional
	Pattern string `json:"pattern"`
	// Extract allows a capture group to be extracted from the specified regular
	// expression pattern, useful before tag evaluation.
	// +optional
	Extract string `json:"extract"`
}

// ImageRef represents an image reference.
type ImageRef struct {
	// Name is the bare image's name.
	// +required
	Name string `json:"name"`
	// Tag is the image's tag.
	// +required
	Tag string `json:"tag"`
	// Digest is the image's digest.
	// +optional
	Digest string `json:"digest,omitempty"`
}

func (in *ImageRef) String() string {
	res := in.Name + ":" + in.Tag
	if in.Digest != "" {
		res += "@" + in.Digest
	}
	return res
}

// ImagePolicyStatus defines the observed state of ImagePolicy
type ImagePolicyStatus struct {
	// LatestRef gives the first in the list of images scanned by
	// the image repository, when filtered and ordered according
	// to the policy.
	LatestRef *ImageRef `json:"latestRef,omitempty"`
	// ObservedPreviousRef is the observed previous LatestRef. It is used
	// to keep track of the previous and current images.
	// +optional
	ObservedPreviousRef *ImageRef `json:"observedPreviousRef,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	meta.ReconcileRequestStatus `json:",inline"`
}

// GetConditions returns the status conditions of the object.
func (in *ImagePolicy) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *ImagePolicy) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=imgpol;imagepol
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.status.latestRef.name`
// +kubebuilder:printcolumn:name="Tag",type=string,JSONPath=`.status.latestRef.tag`
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// ImagePolicy is the Schema for the imagepolicies API
type ImagePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ImagePolicySpec `json:"spec,omitempty"`
	// +kubebuilder:default={"observedGeneration":-1}
	Status ImagePolicyStatus `json:"status,omitempty"`
}

func (in *ImagePolicy) GetDigestReflectionPolicy() ReflectionPolicy {
	if in.Spec.DigestReflectionPolicy != "" {
		return in.Spec.DigestReflectionPolicy
	}
	return ReflectNever
}

func (in *ImagePolicy) GetInterval() time.Duration {
	if in.GetDigestReflectionPolicy() == ReflectAlways {
		if in.Spec.Interval == nil || in.Spec.Interval.Duration == 0 {
			return 10 * time.Minute
		}

		return in.Spec.Interval.Duration
	}

	return 0
}

//+kubebuilder:object:root=true

// ImagePolicyList contains a list of ImagePolicy
type ImagePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImagePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ImagePolicy{}, &ImagePolicyList{})
}
