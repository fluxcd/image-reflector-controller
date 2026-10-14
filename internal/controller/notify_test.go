/*
Copyright 2026 The Flux authors

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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/events"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
)

func TestNotify(t *testing.T) {
	nextScanMsg := "foo"
	tests := []struct {
		name       string
		beforeFunc func(oldObj, newObj *imagev1.ImageRepository)
		wantEvent  *corev1.Event
	}{
		{
			name: "first time success reconcile, empty old object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
			},
			wantEvent: &corev1.Event{
				Type:    corev1.EventTypeNormal,
				Reason:  meta.SucceededReason,
				Action:  imagev1.ActionScan.String(),
				Message: "found x tags",
			},
		},
		{
			name: "no-op reconcile, same old and new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
			},
			wantEvent: &corev1.Event{
				Type:    eventv1.EventTypeTrace,
				Reason:  meta.SucceededReason,
				Action:  imagev1.ActionScan.String(),
				Message: nextScanMsg,
			},
		},
		{
			name: "new tags, ready but different old and new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found y tags")
			},
			wantEvent: &corev1.Event{
				Type:    corev1.EventTypeNormal,
				Reason:  meta.SucceededReason,
				Action:  imagev1.ActionScan.String(),
				Message: "found y tags",
			},
		},
		{
			name: "ready old object, not ready new object",
			beforeFunc: func(oldObj, newObj *imagev1.ImageRepository) {
				conditions.MarkTrue(oldObj, meta.ReadyCondition, meta.SucceededReason, "found x tags")
				conditions.MarkFalse(newObj, meta.ReadyCondition, meta.FailedReason, "scan failed")
			},
			wantEvent: &corev1.Event{
				Type:    corev1.EventTypeWarning,
				Reason:  meta.FailedReason,
				Action:  imagev1.ActionScan.String(),
				Message: "scan failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			recorder := events.NewFakeRecorder(32, false)

			oldObj := &imagev1.ImageRepository{}
			newObj := oldObj.DeepCopy()

			if tt.beforeFunc != nil {
				tt.beforeFunc(oldObj, newObj)
			}

			notify(ctx, recorder, oldObj, newObj, nil, imagev1.ActionScan, nextScanMsg)

			select {
			case x, ok := <-recorder.Events:
				g.Expect(ok).To(BeTrue(), "unexpected closed events channel")
				g.Expect(tt.wantEvent).ToNot(BeNil(), "unexpected event received")
				g.Expect(x.Type).To(Equal(tt.wantEvent.Type))
				g.Expect(x.Reason).To(Equal(tt.wantEvent.Reason))
				g.Expect(x.Action).To(Equal(tt.wantEvent.Action))
				g.Expect(x.Message).To(Equal(tt.wantEvent.Message))
			default:
				g.Expect(tt.wantEvent).To(BeNil(), "expected an event to be emitted")
			}
		})
	}
}

// TestNotifyRelatedObject verifies that notify annotates the emitted event
// with the related object reference when one is provided (e.g. the
// ImageRepository an ImagePolicy selects from), and leaves it unset when the
// related object is nil (early returns such as not found or access denied).
func TestNotifyRelatedObject(t *testing.T) {
	readyMsg := "found latest image"

	tests := []struct {
		name        string
		related     runtime.Object
		wantRelated *corev1.ObjectReference
	}{
		{
			name: "related ImageRepository is recorded on the event",
			related: &imagev1.ImageRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "repo",
					Namespace: "ns",
				},
			},
			wantRelated: &corev1.ObjectReference{
				Name:      "repo",
				Namespace: "ns",
			},
		},
		{
			name:        "nil related object leaves the reference unset",
			related:     nil,
			wantRelated: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// includeObject=true so the fake records the related reference.
			recorder := events.NewFakeRecorder(32, true)

			oldObj := &imagev1.ImagePolicy{}
			newObj := oldObj.DeepCopy()
			// Drive the "became ready from not ready" branch so an event is emitted.
			conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "%s", readyMsg)

			notify(ctx, recorder, oldObj, newObj, tt.related, imagev1.ActionSelectImage, readyMsg)

			recorded := recorder.GetEvents()
			g.Expect(recorded).To(HaveLen(1), "expected exactly one event to be emitted")
			g.Expect(recorded[0].Action).To(Equal(imagev1.ActionSelectImage.String()))

			if tt.wantRelated == nil {
				g.Expect(recorded[0].Related).To(BeNil(), "expected no related object reference")
				return
			}
			g.Expect(recorded[0].Related).ToNot(BeNil(), "expected a related object reference")
			g.Expect(recorded[0].Related.Name).To(Equal(tt.wantRelated.Name))
			g.Expect(recorded[0].Related.Namespace).To(Equal(tt.wantRelated.Namespace))
		})
	}
}

// TestNotifyTypedNilRelatedObject guards against the Go typed-nil-interface
// pitfall: a nil *imagev1.ImageRepository stored in a runtime.Object interface
// is not an interface nil. The reconciler passes an untyped nil in that case,
// which this test models to confirm the related reference stays unset.
func TestNotifyTypedNilRelatedObject(t *testing.T) {
	g := NewWithT(t)

	recorder := events.NewFakeRecorder(32, true)

	oldObj := &imagev1.ImagePolicy{}
	newObj := oldObj.DeepCopy()
	conditions.MarkTrue(newObj, meta.ReadyCondition, meta.SucceededReason, "found latest image")

	// Model the reconciler guard: when the resolved repo pointer is nil, an
	// untyped nil interface is passed rather than a typed-nil pointer.
	var relatedRepo *imagev1.ImageRepository
	var related runtime.Object
	if relatedRepo != nil {
		related = relatedRepo
	}

	notify(ctx, recorder, oldObj, newObj, related, imagev1.ActionSelectImage, "found latest image")

	recorded := recorder.GetEvents()
	g.Expect(recorded).To(HaveLen(1), "expected exactly one event to be emitted")
	g.Expect(recorded[0].Related).To(BeNil(), "expected no related object reference for untyped nil")
}

// expectEvent drains a single event from the FakeRecorder and asserts it
// matches want. The Message is matched by substring so callers can assert on a
// stable fragment without pinning the whole message. A nil want asserts that no
// event was emitted. It is used by the controller reconcile tests to verify the
// event emitted by notify alongside the reconcile outcome.
func expectEvent(g *WithT, recorder *events.FakeRecorder, want *corev1.Event) {
	g.THelper()
	recorded := recorder.GetEvents()
	if want == nil {
		g.Expect(recorded).To(BeEmpty(), "expected no event to be emitted")
		return
	}
	g.Expect(recorded).To(HaveLen(1), "expected exactly one event to be emitted")
	g.Expect(recorded[0].Type).To(Equal(want.Type))
	g.Expect(recorded[0].Reason).To(Equal(want.Reason))
	g.Expect(recorded[0].Action).To(Equal(want.Action))
	g.Expect(recorded[0].Message).To(ContainSubstring(want.Message))
}
