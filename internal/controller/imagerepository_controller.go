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
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	ctrreconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"
	"github.com/fluxcd/pkg/runtime/reconcile"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/registry"
)

// latestTagsCount is the number of tags to use as latest tags.
const latestTagsCount = 10

// imageRepositoryOwnedConditions is a list of conditions owned by the
// ImageRepositoryReconciler.
var imageRepositoryOwnedConditions = []string{
	meta.ReadyCondition,
	meta.ReconcilingCondition,
	meta.StalledCondition,
}

// imageRepositoryNegativeConditions is a list of negative polarity conditions
// owned by ImageRepositoryReconciler. It is used in tests for compliance with
// kstatus.
var imageRepositoryNegativeConditions = []string{
	meta.StalledCondition,
	meta.ReconcilingCondition,
}

// Reasons for scan.
const (
	scanReasonNeverScanned         = "first scan"
	scanReasonReconcileRequested   = "reconcile requested"
	scanReasonNewImageName         = "new image name"
	scanReasonUpdatedExclusionList = "updated exclusion list"
	scanReasonEmptyDatabase        = "no tags in database"
	scanReasonInterval             = "triggered by interval"
)

// getPatchOptions composes patch options based on the given parameters.
// It is used as the options used when patching an object.
func getPatchOptions(ownedConditions []string, controllerName string) []patch.Option {
	return []patch.Option{
		patch.WithOwnedConditions{Conditions: ownedConditions},
		patch.WithFieldOwner(controllerName),
	}
}

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create

// ImageRepositoryReconciler reconciles a ImageRepository object
type ImageRepositoryReconciler struct {
	client.Client
	kuberecorder.EventRecorder
	helper.Metrics

	ControllerName string
	TokenCache     *cache.TokenCache
	Database       interface {
		DatabaseWriter
		DatabaseReader
	}
	AuthOptionsGetter *registry.AuthOptionsGetter

	patchOptions []patch.Option
}

type ImageRepositoryReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[ctrreconcile.Request]
}

func (r *ImageRepositoryReconciler) SetupWithManager(mgr ctrl.Manager, opts ImageRepositoryReconcilerOptions) error {
	r.patchOptions = getPatchOptions(imageRepositoryOwnedConditions, r.ControllerName)

	return ctrl.NewControllerManagedBy(mgr).
		For(&imagev1.ImageRepository{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).
		Complete(r)
}

func (r *ImageRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	start := time.Now()
	log := ctrl.LoggerFrom(ctx)

	// Fetch the ImageRepository.
	obj := &imagev1.ImageRepository{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the patch helper with the current version of the object.
	serialPatcher := patch.NewSerialPatcher(obj, r.Client)

	// Always attempt to patch the object after each reconciliation.
	defer func() {
		// If the reconcile request annotation was set, consider it
		// handled (NB it doesn't matter here if it was changed since last
		// time)
		if token, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
			obj.Status.SetLastHandledReconcileRequest(token)
		}

		// Create patch options for the final patch of the object.
		patchOpts := reconcile.AddPatchOptions(obj, r.patchOptions, imageRepositoryOwnedConditions, r.ControllerName)
		if err := serialPatcher.Patch(ctx, obj, patchOpts...); err != nil {
			// Ignore patch error "not found" when the object is being deleted.
			if !obj.GetDeletionTimestamp().IsZero() {
				err = kerrors.FilterOut(err, func(e error) bool { return apierrors.IsNotFound(e) })
			}
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		// Always record duration metrics.
		r.Metrics.RecordDuration(ctx, obj, start)
	}()

	// Examine if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(obj)
	}

	// Add finalizer first if it doesn't exist to avoid the race condition
	// between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp
	// is not set.
	if !controllerutil.ContainsFinalizer(obj, imagev1.ImageFinalizer) {
		controllerutil.AddFinalizer(obj, imagev1.ImageFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	// Return if the object is suspended.
	if obj.Spec.Suspend {
		log.Info("reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	// Call subreconciler.
	result, retErr = r.reconcile(ctx, serialPatcher, obj, start)
	return
}

func (r *ImageRepositoryReconciler) reconcile(ctx context.Context, sp *patch.SerialPatcher,
	obj *imagev1.ImageRepository, startTime time.Time) (result ctrl.Result, retErr error) {
	oldObj := obj.DeepCopy()

	var tagsChecksum string
	var numFoundTags int
	// Store a message about current reconciliation and next scan.
	var nextScanMsg string
	// Set a default next scan time before processing the object.
	nextScanTime := obj.GetRequeueAfter()

	defer func() {
		// Define the meaning of success based on the value of next scan time.
		isSuccess := func(res ctrl.Result, err error) bool {
			if err != nil || res.RequeueAfter != nextScanTime || res.Requeue {
				return false
			}
			return true
		}

		readyMsg := fmt.Sprintf("successful scan: found %d tags with checksum %s", numFoundTags, tagsChecksum)
		rs := reconcile.NewResultFinalizer(isSuccess, readyMsg)
		retErr = rs.Finalize(obj, result, retErr)

		// Presence of reconciling means that the reconciliation didn't succeed.
		// Set the Reconciling reason to ProgressingWithRetry to indicate a
		// failure retry.
		if conditions.IsReconciling(obj) {
			reconciling := conditions.Get(obj, meta.ReconcilingCondition)
			reconciling.Reason = meta.ProgressingWithRetryReason
			conditions.Set(obj, reconciling)
		}

		notify(ctx, r.EventRecorder, oldObj, obj, nextScanMsg)
	}()

	// Check object-level workload identity feature gate.
	if obj.Spec.Provider != "generic" && obj.Spec.ServiceAccountName != "" && !auth.IsObjectLevelWorkloadIdentityEnabled() {
		const msgFmt = "to use spec.serviceAccountName for provider authentication please enable the %s feature gate in the controller"
		conditions.MarkStalled(obj, meta.FeatureGateDisabledReason, msgFmt,
			auth.FeatureGateObjectLevelWorkloadIdentity)
		result, retErr = ctrl.Result{}, nil
		return
	}

	// Set reconciling condition.
	reconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress")

	var reconcileAtVal string
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		reconcileAtVal = v
	}

	// Persist reconciling if generation differs or reconciliation is requested.
	switch {
	case obj.Generation != obj.Status.ObservedGeneration:
		reconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
		if err := sp.Patch(ctx, obj, r.patchOptions...); err != nil {
			result, retErr = ctrl.Result{}, err
			return
		}
	case reconcileAtVal != obj.Status.GetLastHandledReconcileRequest():
		if err := sp.Patch(ctx, obj, r.patchOptions...); err != nil {
			result, retErr = ctrl.Result{}, err
			return
		}
	}

	// Parse image reference.
	ref, err := registry.ParseImageReference(obj.Spec.Image, obj.Spec.Insecure)
	if err != nil {
		conditions.MarkStalled(obj, imagev1.ImageURLInvalidReason, "%s", err)
		result, retErr = ctrl.Result{}, nil
		return
	}
	conditions.Delete(obj, meta.StalledCondition)

	involvedObject := &cache.InvolvedObject{
		Kind:      imagev1.ImageRepositoryKind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Operation: cache.OperationReconcile,
	}
	opts, err := r.AuthOptionsGetter.GetOptions(ctx, obj, involvedObject)
	if err != nil {
		e := fmt.Errorf("failed to configure authentication options: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, imagev1.AuthenticationFailedReason, "%s", e)
		result, retErr = ctrl.Result{}, e
		return
	}

	// Check if it can be scanned now.
	ok, when, reasonMsg, err := r.shouldScan(*obj, startTime)
	if err != nil {
		e := fmt.Errorf("failed to determine if it's scan time: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, metav1.StatusFailure, "%s", e)
		result, retErr = ctrl.Result{}, e
		return
	}

	// Scan the repository if it's scan time. No scan is a no-op reconciliation.
	// The next scan time is not reset in case of no-op reconciliation.
	if ok {
		// When the database is empty, we need to set the Ready condition to
		// Unknown to force a transition when the controller restarts.
		// This transition is required to unblock the ImagePolicy object
		// from the Ready condition stuck in False with reason DependencyNotReady.
		drift := reasonMsg == scanReasonEmptyDatabase
		reconcile.ProgressiveStatus(drift, obj, meta.ProgressingReason, "scanning: %s", reasonMsg)
		if err := sp.Patch(ctx, obj, r.patchOptions...); err != nil {
			result, retErr = ctrl.Result{}, err
			return
		}

		if err := r.scan(ctx, obj, ref, opts); err != nil {
			e := fmt.Errorf("scan failed: %w", err)
			conditions.MarkFalse(obj, meta.ReadyCondition, imagev1.ReadOperationFailedReason, "%s", e)
			result, retErr = ctrl.Result{}, e
			return
		}

		nextScanMsg = fmt.Sprintf("next scan in %s", when.String())
		// Check if new tags were found.
		if oldObj.Status.LastScanResult != nil &&
			oldObj.Status.LastScanResult.Revision == obj.Status.LastScanResult.Revision {
			nextScanMsg = "tags did not change, " + nextScanMsg
		} else {
			// When new tags are found, this message will be suppressed by
			// another event based on the new Ready=true status value. This is
			// set as a default message.
			nextScanMsg = "successful scan, " + nextScanMsg
		}
	} else {
		nextScanMsg = fmt.Sprintf("no change in repository configuration since last scan, next scan in %s", when.String())
	}
	tagsChecksum = obj.Status.LastScanResult.Revision
	numFoundTags = obj.Status.LastScanResult.TagCount

	// Set the observations on the status.
	obj.Status.CanonicalImageName = ref.Context().String()
	obj.Status.ObservedExclusionList = obj.GetExclusionList()

	// Let result finalizer compute the Ready condition.
	conditions.Delete(obj, meta.ReadyCondition)

	// Set the next scan time in the result.
	nextScanTime = when
	result, retErr = ctrl.Result{RequeueAfter: when}, nil
	return
}

// shouldScan takes an image repo and the time now, and returns whether
// the repository should be scanned now, and how long to wait for the
// next scan. It also returns the reason for the scan.
// It returns immediate scan if
//   - the repository is never scanned before
//   - reconcile annotation is set on the object with a new value
//   - the image URL has changed
//   - the exclusion list has changed
//   - there's no tag in the database
//   - the difference between current time and last time is more than the scan
//     interval
//
// Else it returns with next scan time.
func (r *ImageRepositoryReconciler) shouldScan(obj imagev1.ImageRepository, now time.Time) (bool, time.Duration, string, error) {
	scanInterval := obj.Spec.Interval.Duration

	// Never scanned; do it now.
	lastScanResult := obj.Status.LastScanResult
	if lastScanResult == nil {
		return true, scanInterval, scanReasonNeverScanned, nil
	}
	lastScanTime := lastScanResult.ScanTime

	// Is the controller seeing this because the reconcileAt
	// annotation was tweaked? Despite the name of the annotation, all
	// that matters is that it's different.
	if syncAt, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		if syncAt != obj.Status.GetLastHandledReconcileRequest() {
			return true, scanInterval, scanReasonReconcileRequested, nil
		}
	}

	// If the canonical image name of the image is different from the last
	// observed name, scan now.
	ref, err := registry.ParseImageReference(obj.Spec.Image, obj.Spec.Insecure)
	if err != nil {
		return false, scanInterval, "", err
	}
	if ref.Context().String() != obj.Status.CanonicalImageName {
		return true, scanInterval, scanReasonNewImageName, nil
	}

	// If the exclusion list has changed, scan now.
	if !isEqualSliceContent(obj.GetExclusionList(), obj.Status.ObservedExclusionList) {
		return true, scanInterval, scanReasonUpdatedExclusionList, nil
	}

	// when recovering, it's possible that the resource has a last
	// scan time, but there's no records because the database has been
	// dropped and created again.

	// FIXME If the repo exists, has been
	// scanned, and doesn't have any tags, this will mean a scan every
	// time the resource comes up for reconciliation.
	tags, err := r.Database.Tags(obj.Status.CanonicalImageName)
	if err != nil {
		return false, scanInterval, "", err
	}
	if len(tags) == 0 {
		return true, scanInterval, scanReasonEmptyDatabase, nil
	}

	when := scanInterval - now.Sub(lastScanTime.Time)
	if when < time.Second {
		return true, scanInterval, scanReasonInterval, nil
	}
	return false, when, "", nil
}

// scan performs repository scanning and writes the scanned result in the
// internal database and populates the status of the ImageRepository.
func (r *ImageRepositoryReconciler) scan(ctx context.Context, obj *imagev1.ImageRepository, ref name.Reference, options []remote.Option) error {
	timeout := obj.GetTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	options = append(options, remote.WithContext(ctx))

	tags, err := remote.List(ref.Context(), options...)
	if err != nil {
		return err
	}

	filteredTags, err := filterOutTags(tags, obj.GetExclusionList())
	if err != nil {
		return err
	}

	latestTags := sortTagsAndGetLatestTags(filteredTags)
	if len(latestTags) == 0 {
		latestTags = nil // for omission in json serialization when empty
	}

	canonicalName := ref.Context().String()
	checksum, err := r.Database.SetTags(canonicalName, filteredTags)
	if err != nil {
		return fmt.Errorf("failed to set tags for %q: %w", canonicalName, err)
	}

	obj.Status.LastScanResult = &imagev1.ScanResult{
		Revision:   checksum,
		TagCount:   len(filteredTags),
		ScanTime:   metav1.Now(),
		LatestTags: latestTags,
	}

	return nil
}

// reconcileDelete handles the deletion of the object.
func (r *ImageRepositoryReconciler) reconcileDelete(obj *imagev1.ImageRepository) (ctrl.Result, error) {
	// Remove our finalizer from the list.
	controllerutil.RemoveFinalizer(obj, imagev1.ImageFinalizer)

	// Cleanup caches.
	r.TokenCache.DeleteEventsForObject(imagev1.ImageRepositoryKind,
		obj.GetName(), obj.GetNamespace(), cache.OperationReconcile)

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

// eventLogf records events, and logs at the same time.
//
// This log is different from the debug log in the EventRecorder, in the sense
// that this is a simple log. While the debug log contains complete details
// about the event.
func eventLogf(ctx context.Context, r kuberecorder.EventRecorder, obj runtime.Object, eventType string, reason string, messageFmt string, args ...interface{}) {
	msg := fmt.Sprintf(messageFmt, args...)
	// Log and emit event.
	if eventType == corev1.EventTypeWarning {
		ctrl.LoggerFrom(ctx).Error(errors.New(reason), msg)
	} else {
		ctrl.LoggerFrom(ctx).Info(msg)
	}
	r.Eventf(obj, eventType, reason, msg)
}

// filterOutTags filters the given tags through the given regular expression
// patterns and returns a list of tags that don't match with the pattern.
func filterOutTags(tags []string, patterns []string) ([]string, error) {
	// Compile all the regex first.
	compiledRegexp := []*regexp.Regexp{}
	for _, pattern := range patterns {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex %s: %w", pattern, err)
		}
		compiledRegexp = append(compiledRegexp, r)
	}

	// Pass the tags through the compiled regex and collect the filtered tags.
	filteredTags := []string{}
	for _, tag := range tags {
		match := false
		for _, regex := range compiledRegexp {
			if regex.MatchString(tag) {
				match = true
				break
			}
		}
		if !match {
			filteredTags = append(filteredTags, tag)
		}
	}
	return filteredTags, nil
}

// sortTagsAndGetLatestTags takes a slice of tags, sorts them in-place
// in descending order of their values and returns the 10 latest tags.
func sortTagsAndGetLatestTags(tags []string) []string {
	slices.SortStableFunc(tags, func(a, b string) int { return -strings.Compare(a, b) })
	latestTags := tags[:min(len(tags), latestTagsCount)]
	// We can't return a slice of the original slice here because the original
	// slice can be too large and we want to free up that memory. Our copy has
	// at most latestTagsCount elements, which is specifically a small number.
	return slices.Clone(latestTags)
}

// isEqualSliceContent compares two string slices to check if they have the same
// content.
func isEqualSliceContent(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// notify emits events, logs and notification based on the resulting objects
// before and after the reconciliation.
func notify(ctx context.Context, r kuberecorder.EventRecorder, oldObj, newObj conditions.Setter, nextScanMsg string) {
	ready := conditions.Get(newObj, meta.ReadyCondition)

	// Was ready before and is ready now, but the scan results have changed.
	if conditions.IsReady(oldObj) && conditions.IsReady(newObj) &&
		(conditions.GetMessage(oldObj, meta.ReadyCondition)) != ready.Message {
		eventLogf(ctx, r, newObj, corev1.EventTypeNormal, ready.Reason, "%s", ready.Message)
		return
	}

	// Emit events when reconciliation fails or recovers from failure.

	// Became ready from not ready.
	if !conditions.IsReady(oldObj) && conditions.IsReady(newObj) {
		eventLogf(ctx, r, newObj, corev1.EventTypeNormal, ready.Reason, "%s", ready.Message)
		return
	}
	// Not ready, failed.
	if !conditions.IsReady(newObj) {
		eventLogf(ctx, r, newObj, corev1.EventTypeWarning, ready.Reason, "%s", ready.Message)
		return
	}

	eventLogf(ctx, r, newObj, eventv1.EventTypeTrace, meta.SucceededReason, "%s", nextScanMsg)
}
