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

			notify(ctx, recorder, oldObj, newObj, imagev1.ActionScan, nextScanMsg)

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
