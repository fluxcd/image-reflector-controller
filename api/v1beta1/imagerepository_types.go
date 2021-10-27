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

package v1beta1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
)

const ImageRepositoryKind = "ImageRepository"

// ImageRepositorySpec defines the parameters for scanning an image
// repository, e.g., `fluxcd/flux`.
type ImageRepositorySpec struct {
	// Image is the name of the image repository
	// +required
	Image string `json:"image,omitempty"`
	// Interval is the length of time to wait between
	// scans of the image repository.
	// +required
	Interval metav1.Duration `json:"interval,omitempty"`

	// Timeout for image scanning.
	// Defaults to 'Interval' duration.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// SecretRef can be given the name of a secret containing
	// credentials to use for the image registry. The secret should be
	// created with `kubectl create secret docker-registry`, or the
	// equivalent.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// CertSecretRef can be given the name of a secret containing
	// either or both of
	//
	//  - a PEM-encoded client certificate (`certFile`) and private
	//  key (`keyFile`);
	//  - a PEM-encoded CA certificate (`caFile`)
	//
	//  and whichever are supplied, will be used for connecting to the
	//  registry. The client cert and key are useful if you are
	//  authenticating with a certificate; the CA cert is useful if
	//  you are using a self-signed server certificate.
	// +optional
	CertSecretRef *meta.LocalObjectReference `json:"certSecretRef,omitempty"`

	// This flag tells the controller to suspend subsequent image scans.
	// It does not apply to already started scans. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// AccessFrom defines an ACL for allowing cross-namespace references
	// to the ImageRepository object based on the caller's namespace labels.
	// +optional
	AccessFrom *AccessFrom `json:"accessFrom,omitempty"`
}

type AccessFrom struct {
	NamespaceSelectors []NamespaceSelector `json:"namespaceSelectors,omitempty"`
}

type NamespaceSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type ScanResult struct {
	TagCount int         `json:"tagCount"`
	ScanTime metav1.Time `json:"scanTime,omitempty"`
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

	meta.ReconcileRequestStatus `json:",inline"`
}

// SetImageRepositoryReadiness sets the ready condition with the given status, reason and message.
func SetImageRepositoryReadiness(ir *ImageRepository, status metav1.ConditionStatus, reason, message string) {
	ir.Status.ObservedGeneration = ir.ObjectMeta.Generation
	meta.SetResourceCondition(ir, meta.ReadyCondition, status, reason, message)
}

// GetStatusConditions returns a pointer to the Status.Conditions slice
func (in *ImageRepository) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
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

// +kubebuilder:object:root=true

// ImageRepositoryList contains a list of ImageRepository
type ImageRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ImageRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ImageRepository{}, &ImageRepositoryList{})
}
