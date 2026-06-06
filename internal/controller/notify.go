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
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/events"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
)

// eventLogf records events, and logs at the same time.
//
// This log is different from the debug log in the Recorder, in the sense
// that this is a simple log. While the debug log contains complete details
// about the event.
func eventLogf(ctx context.Context, r events.Recorder, obj runtime.Object, related runtime.Object, eventType string, reason string, action imagev1.Action, messageFmt string, args ...interface{}) {
	msg := fmt.Sprintf(messageFmt, args...)
	// Log and emit event.
	if eventType == corev1.EventTypeWarning {
		ctrl.LoggerFrom(ctx).Error(errors.New(reason), msg)
	} else {
		ctrl.LoggerFrom(ctx).Info(msg)
	}
	r.Eventf(obj, related, eventType, reason, action.String(), "%s", msg)
}

// notify emits events, logs and notification based on the resulting objects
// before and after the reconciliation. It is shared by the ImageRepository
// and ImagePolicy reconcilers. The statusMsg carries a reconciler-specific
// trace message for no-op reconciles.
func notify(ctx context.Context, r events.Recorder, oldObj, newObj conditions.Setter, action imagev1.Action, statusMsg string) {
	ready := conditions.Get(newObj, meta.ReadyCondition)

	// Was ready before and is ready now, but the results have changed.
	if conditions.IsReady(oldObj) && conditions.IsReady(newObj) &&
		(conditions.GetMessage(oldObj, meta.ReadyCondition)) != ready.Message {
		eventLogf(ctx, r, newObj, oldObj, corev1.EventTypeNormal, ready.Reason, action, "%s", ready.Message)
		return
	}

	// Emit events when reconciliation fails or recovers from failure.

	// Became ready from not ready.
	if !conditions.IsReady(oldObj) && conditions.IsReady(newObj) {
		eventLogf(ctx, r, newObj, oldObj, corev1.EventTypeNormal, ready.Reason, action, "%s", ready.Message)
		return
	}
	// Not ready, failed.
	if !conditions.IsReady(newObj) {
		eventLogf(ctx, r, newObj, oldObj, corev1.EventTypeWarning, ready.Reason, action, "%s", ready.Message)
		return
	}
	eventLogf(ctx, r, newObj, oldObj, eventv1.EventTypeTrace, meta.SucceededReason, action, "%s", statusMsg)
}
