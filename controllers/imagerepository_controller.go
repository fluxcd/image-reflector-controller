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

package controllers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/oci/auth/login"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"
	"github.com/fluxcd/pkg/runtime/reconcile"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/secret"
)

// imageRepositoryOwnedConditions is a list of conditions owned by the
// ImageRepositoryReconciler.
var imageRepositoryOwnedConditions = []string{
	meta.ReadyCondition,
	meta.ReconcilingCondition,
	meta.StalledCondition,
}

// Reasons for scan.
const (
	scanReasonNeverScanned         = "never scanned before"
	scanReasonReconcileRequested   = "reconcile requested"
	scanReasonNewImageName         = "new image name"
	scanReasonUpdatedExclusionList = "updated exclusion list"
	scanReasonEmptyDatabase        = "no tags in database"
	scanReasonInterval             = "scan interval"
)

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch

// ImageRepositoryReconciler reconciles a ImageRepository object
type ImageRepositoryReconciler struct {
	client.Client
	kuberecorder.EventRecorder
	helper.Metrics

	ControllerName        string
	ExternalEventRecorder *events.Recorder
	Database              interface {
		DatabaseWriter
		DatabaseReader
	}
	login.ProviderOptions
}

type ImageRepositoryReconcilerOptions struct {
	MaxConcurrentReconciles int
	RateLimiter             ratelimiter.RateLimiter
}

func (r *ImageRepositoryReconciler) SetupWithManager(mgr ctrl.Manager, opts ImageRepositoryReconcilerOptions) error {
	recoverPanic := true
	return ctrl.NewControllerManagedBy(mgr).
		For(&imagev1.ImageRepository{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: opts.MaxConcurrentReconciles,
			RateLimiter:             opts.RateLimiter,
			RecoverPanic:            &recoverPanic,
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

	// Record suspended status metric.
	r.RecordSuspend(ctx, obj, obj.Spec.Suspend)

	// Initialize the patch helper with the current version of the object.
	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to patch the object after each reconciliation.
	defer func() {
		// Create patch options for patching the object.
		patchOpts := []patch.Option{}
		patchOpts = reconcile.AddPatchOptions(obj, patchOpts, imageRepositoryOwnedConditions, r.ControllerName)
		if err = patchHelper.Patch(ctx, obj, patchOpts...); err != nil {
			// Ignore patch error "not found" when the object is being deleted.
			if !obj.GetDeletionTimestamp().IsZero() {
				err = kerrors.FilterOut(err, func(e error) bool { return apierrors.IsNotFound(e) })
			}
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		// Always record readiness and duration metrics.
		r.Metrics.RecordReadiness(ctx, obj)
		r.Metrics.RecordDuration(ctx, obj, start)
	}()

	// Add finalizer first if it doesn't exist to avoid the race condition
	// between init and delete.
	if !controllerutil.ContainsFinalizer(obj, imagev1.ImageRepositoryFinalizer) {
		controllerutil.AddFinalizer(obj, imagev1.ImageRepositoryFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	// Examine if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, obj)
	}

	// Return if the object is suspended.
	if obj.Spec.Suspend {
		log.Info("reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	// Call subreconciler.
	result, retErr = r.reconcile(ctx, obj, start)
	return
}

func (r *ImageRepositoryReconciler) reconcile(ctx context.Context, obj *imagev1.ImageRepository, startTime time.Time) (result ctrl.Result, retErr error) {
	oldObj := obj.DeepCopy()

	var foundTags int
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

		readyMsg := fmt.Sprintf("successful scan, found %d tags", foundTags)
		rs := reconcile.NewResultFinalizer(isSuccess, readyMsg)
		retErr = rs.Finalize(obj, result, retErr)

		notify(ctx, r.EventRecorder, oldObj, obj, nextScanMsg)
	}()

	// Set reconciling condition.
	if obj.Generation != obj.Status.ObservedGeneration {
		conditions.MarkReconciling(obj, "NewGeneration", "reconciling new object generation (%d)", obj.Generation)
	}

	// Clear previous ready status condition value.
	conditions.Delete(obj, meta.ReadyCondition)

	// Parse image reference.
	ref, err := parseImageReference(obj.Spec.Image)
	if err != nil {
		conditions.MarkStalled(obj, imagev1.ImageURLInvalidReason, err.Error())
		result, retErr = ctrl.Result{}, nil
		return
	}
	conditions.Delete(obj, meta.StalledCondition)

	opts, err := r.setAuthOptions(ctx, obj, ref)
	if err != nil {
		e := fmt.Errorf("failed to configure authentication options: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, imagev1.AuthenticationFailedReason, e.Error())
		result, retErr = ctrl.Result{}, e
		return
	}

	// Check if it can be scanned now.
	ok, when, reasonMsg, err := r.shouldScan(*obj, startTime)
	if err != nil {
		e := fmt.Errorf("failed to determine if it's scan time: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, metav1.StatusFailure, e.Error())
		result, retErr = ctrl.Result{}, e
		return
	}

	// Scan the repository if it's scan time. No scan is a no-op reconciliation.
	// The next scan time is not reset in case of no-op reconciliation.
	if ok {
		conditions.MarkReconciling(obj, "Scanning", reasonMsg)
		tags, err := r.scan(ctx, obj, ref, opts)
		if err != nil {
			e := fmt.Errorf("scan failed: %w", err)
			conditions.MarkFalse(obj, meta.ReadyCondition, imagev1.ReadOperationFailedReason, e.Error())
			result, retErr = ctrl.Result{}, e
			return
		}
		foundTags = tags

		nextScanMsg = fmt.Sprintf("next scan in %s", when.String())
		// Check if new tags were found.
		if oldObj.Status.LastScanResult != nil &&
			oldObj.Status.LastScanResult.TagCount == foundTags {
			nextScanMsg = "no new tags found, " + nextScanMsg
		} else {
			// When new tags are found, this message will be suppressed by
			// another event based on the new Ready=true status value. This is
			// set as a default message.
			nextScanMsg = "successful scan, " + nextScanMsg
		}
	} else {
		foundTags = obj.Status.LastScanResult.TagCount
		nextScanMsg = fmt.Sprintf("no change in repository configuration since last scan, next scan in %s", when.String())
	}

	// Set the observations on the status.
	obj.Status.CanonicalImageName = ref.Context().String()
	obj.Status.ObservedExclusionList = obj.GetExclusionList()

	// Remove any stale Ready condition, most likely False, set above. Its value
	// is derived from the overall result of the reconciliation in the deferred
	// block at the very end.
	conditions.Delete(obj, meta.ReadyCondition)

	// Set the next scan time in the result.
	nextScanTime = when
	result, retErr = ctrl.Result{RequeueAfter: when}, nil
	return
}

// setAuthOptions returns authentication options required to scan a repository.
func (r *ImageRepositoryReconciler) setAuthOptions(ctx context.Context, obj *imagev1.ImageRepository, ref name.Reference) ([]remote.Option, error) {
	timeout := obj.GetTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Configure authentication strategy to access the registry.
	var options []remote.Option
	var authSecret corev1.Secret
	var auth authn.Authenticator
	var authErr error

	if obj.Spec.SecretRef != nil {
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.Spec.SecretRef.Name,
		}, &authSecret); err != nil {
			return nil, err
		}
		auth, authErr = secret.AuthFromSecret(authSecret, ref)
	} else {
		// Use the registry provider options to attempt registry login.
		auth, authErr = login.NewManager().Login(ctx, obj.Spec.Image, ref, r.ProviderOptions)
	}
	if authErr != nil {
		// If it's not unconfigured provider error, abort reconciliation.
		// Continue reconciliation if it's unconfigured providers for scanning
		// public repositories.
		if !errors.Is(authErr, oci.ErrUnconfiguredProvider) {
			return nil, authErr
		}
	}
	if auth != nil {
		options = append(options, remote.WithAuth(auth))
	}

	// Load any provided certificate.
	if obj.Spec.CertSecretRef != nil {
		var certSecret corev1.Secret
		if obj.Spec.SecretRef != nil && obj.Spec.SecretRef.Name == obj.Spec.CertSecretRef.Name {
			certSecret = authSecret
		} else {
			if err := r.Get(ctx, types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      obj.Spec.CertSecretRef.Name,
			}, &certSecret); err != nil {
				return nil, err
			}
		}

		tr, err := secret.TransportFromSecret(&certSecret)
		if err != nil {
			return nil, err
		}
		options = append(options, remote.WithTransport(tr))
	}

	if obj.Spec.ServiceAccountName != "" {
		serviceAccount := corev1.ServiceAccount{}
		// Lookup service account
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.Spec.ServiceAccountName,
		}, &serviceAccount); err != nil {
			return nil, err
		}

		if len(serviceAccount.ImagePullSecrets) > 0 {
			imagePullSecrets := make([]corev1.Secret, len(serviceAccount.ImagePullSecrets))
			for i, ips := range serviceAccount.ImagePullSecrets {
				var saAuthSecret corev1.Secret
				if err := r.Get(ctx, types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      ips.Name,
				}, &saAuthSecret); err != nil {
					return nil, err
				}
				imagePullSecrets[i] = saAuthSecret
			}
			keychain, err := k8schain.NewFromPullSecrets(ctx, imagePullSecrets)
			if err != nil {
				return nil, err
			}
			options = append(options, remote.WithAuthFromKeychain(keychain))
		}
	}

	return options, nil
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
	ref, err := parseImageReference(obj.Spec.Image)
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
func (r *ImageRepositoryReconciler) scan(ctx context.Context, obj *imagev1.ImageRepository, ref name.Reference, options []remote.Option) (int, error) {
	timeout := obj.GetTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	options = append(options, remote.WithContext(ctx))

	tags, err := remote.List(ref.Context(), options...)
	if err != nil {
		return 0, err
	}

	filteredTags, err := filterOutTags(tags, obj.GetExclusionList())
	if err != nil {
		return 0, err
	}

	canonicalName := ref.Context().String()
	if err := r.Database.SetTags(canonicalName, filteredTags); err != nil {
		return 0, fmt.Errorf("failed to set tags for %q: %w", canonicalName, err)
	}

	scanTime := metav1.Now()
	obj.Status.LastScanResult = &imagev1.ScanResult{
		TagCount: len(filteredTags),
		ScanTime: scanTime,
	}

	// If the reconcile request annotation was set, consider it
	// handled (NB it doesn't matter here if it was changed since last
	// time)
	if token, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		obj.Status.SetLastHandledReconcileRequest(token)
	}

	return len(filteredTags), nil
}

// reconcileDelete handles the deletion of the object.
func (r *ImageRepositoryReconciler) reconcileDelete(ctx context.Context, obj *imagev1.ImageRepository) (ctrl.Result, error) {
	// Remove our finalizer from the list.
	controllerutil.RemoveFinalizer(obj, imagev1.ImageRepositoryFinalizer)

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

// parseImageReference parses the given URL into a container registry repository
// reference.
func parseImageReference(url string) (name.Reference, error) {
	if s := strings.Split(url, "://"); len(s) > 1 {
		return nil, fmt.Errorf(".spec.image value should not start with URL scheme; remove '%s://'", s[0])
	}

	ref, err := name.ParseReference(url)
	if err != nil {
		return nil, err
	}

	imageName := strings.TrimPrefix(url, ref.Context().RegistryStr())
	if s := strings.Split(imageName, ":"); len(s) > 1 {
		return nil, fmt.Errorf(".spec.image value should not contain a tag; remove ':%s'", s[1])
	}

	return ref, nil
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
		eventLogf(ctx, r, newObj, corev1.EventTypeNormal, ready.Reason, ready.Message)
		return
	}

	// Emit events when reconciliation fails or recovers from failure.

	// Became ready from not ready.
	if !conditions.IsReady(oldObj) && conditions.IsReady(newObj) {
		eventLogf(ctx, r, newObj, corev1.EventTypeNormal, ready.Reason, ready.Message)
		return
	}
	// Not ready, failed.
	if !conditions.IsReady(newObj) {
		eventLogf(ctx, r, newObj, corev1.EventTypeWarning, ready.Reason, ready.Message)
		return
	}

	eventLogf(ctx, r, newObj, eventv1.EventTypeTrace, meta.SucceededReason, nextScanMsg)
}
