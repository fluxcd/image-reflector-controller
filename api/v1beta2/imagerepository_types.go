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

package v1beta2

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
)

const ImageRepositoryKind = "ImageRepository"

// Deprecated: Use ImageFinalizer.
const ImageRepositoryFinalizer = "finalizers.fluxcd.io"

// ImageRepositorySpec defines the parameters for scanning an image
// repository, e.g., `fluxcd/flux`.
type ImageRepositorySpec struct {
	// Image is the name of the image repository
	// +required
	Image string `json:"image,omitempty"`
	// Interval is the length of time to wait between
	// scans of the image repository.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +required
	Interval metav1.Duration `json:"interval,omitempty"`

	// Timeout for image scanning.
	// Defaults to 'Interval' duration.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m))+$"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// SecretRef can be given the name of a secret containing
	// credentials to use for the image registry. The secret should be
	// created with `kubectl create secret docker-registry`, or the
	// equivalent.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// ServiceAccountName is the name of the Kubernetes ServiceAccount used to authenticate
	// the image pull if the service account has attached pull secrets.
	// +kubebuilder:validation:MaxLength=253
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// CertSecretRef can be given the name of a Secret containing
	// either or both of
	//
	// - a PEM-encoded client certificate (`tls.crt`) and private
	// key (`tls.key`);
	// - a PEM-encoded CA certificate (`ca.crt`)
	//
	// and whichever are supplied, will be used for connecting to the
	// registry. The client cert and key are useful if you are
	// authenticating with a certificate; the CA cert is useful if
	// you are using a self-signed server certificate. The Secret must
	// be of type `Opaque` or `kubernetes.io/tls`.
	//
	// Note: Support for the `caFile`, `certFile` and `keyFile` keys has
	// been deprecated.
	// +optional
	CertSecretRef *meta.LocalObjectReference `json:"certSecretRef,omitempty"`

	// This flag tells the controller to suspend subsequent image scans.
	// It does not apply to already started scans. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// AccessFrom defines an ACL for allowing cross-namespace references
	// to the ImageRepository object based on the caller's namespace labels.
	// +optional
	AccessFrom *acl.AccessFrom `json:"accessFrom,omitempty"`

	// ExclusionList is a list of regex strings used to exclude certain tags
	// from being stored in the database.
	// +kubebuilder:default:={"^.*\\.sig$"}
	// +kubebuilder:validation:MaxItems:=25
	// +optional
	ExclusionList []string `json:"exclusionList,omitempty"`

	// The provider used for authentication, can be 'aws', 'azure', 'gcp' or 'generic'.
	// When not specified, defaults to 'generic'.
	// +kubebuilder:validation:Enum=generic;aws;azure;gcp
	// +kubebuilder:default:=generic
	// +optional
	Provider string `json:"provider,omitempty"`

	// Insecure, if set to true indicates that the image registry is hosted at an
	// HTTP endpoint.
	// +optional
	Insecure bool `json:"insecure,omitempty"`
}

type ScanResult struct {
	TagCount   int         `json:"tagCount"`
	ScanTime   metav1.Time `json:"scanTime,omitempty"`
	LatestTags []string    `json:"latestTags,omitempty"`
}

// ImageRepositoryStatus defines the observed state of ImageRepository
type ImageRepositoryStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// CanonicalName is the name of the image repository with all the
	// implied bits made explicit; e.g., `docker.io/library/alpine`
	// rather than `alpine`.
	// +optional
	CanonicalImageName string `json:"canonicalImageName,omitempty"`

	// LastScanResult contains the number of fetched tags.
	// +optional
	LastScanResult *ScanResult `json:"lastScanResult,omitempty"`

	// ObservedExclusionList is a list of observed exclusion list. It reflects
	// the exclusion rules used for the observed scan result in
	// spec.lastScanResult.
	ObservedExclusionList []string `json:"observedExclusionList,omitempty"`

	meta.ReconcileRequestStatus `json:",inline"`
}

// GetTimeout returns the timeout with default.
func (in ImageRepository) GetTimeout() time.Duration {
	duration := in.Spec.Interval.Duration
	if in.Spec.Timeout != nil {
		duration = in.Spec.Timeout.Duration
	}
	if duration < time.Second {
		return time.Second
	}
	return duration
}

// GetExclusionList returns the exclusion list with default.
func (in ImageRepository) GetExclusionList() []string {
	el := []string{"^.*\\.sig$"}
	if len(in.Spec.ExclusionList) > 0 {
		el = in.Spec.ExclusionList
	}
	return el
}

// GetProvider returns the provider with default.
func (in ImageRepository) GetProvider() string {
	p := "generic"
	if in.Spec.Provider != "" {
		p = in.Spec.Provider
	}
	return p
}

// GetConditions returns the status conditions of the object.
func (in ImageRepository) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *ImageRepository) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the ImageRepository must be
// reconciled again.
func (in ImageRepository) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Last scan",type=string,JSONPath=`.status.lastScanResult.scanTime`
// +kubebuilder:printcolumn:name="Tags",type=string,JSONPath=`.status.lastScanResult.tagCount`

// ImageRepository is the Schema for the imagerepositories API
type ImageRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ImageRepositorySpec `json:"spec,omitempty"`
	// +kubebuilder:default={"observedGeneration":-1}
	Status ImageRepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ImageRepositoryList contains a list of ImageRepository
type ImageRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImageRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ImageRepository{}, &ImageRepositoryList{})
}
