/*
Copyright 2020, 2021 The Flux authors

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
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	aclapi "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/runtime/predicates"
	pkgreconcile "github.com/fluxcd/pkg/runtime/reconcile"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1"
	"github.com/fluxcd/image-reflector-controller/internal/policy"
	"github.com/fluxcd/image-reflector-controller/internal/registry"
)

// errAccessDenied is returned when an ImageRepository reference in ImagePolicy
// is not allowed.
type errAccessDenied struct {
	err error
}

// Error implements the error interface.
func (e errAccessDenied) Error() string {
	return e.err.Error()
}

// errInvalidPolicy is returned when the policy is invalid and can't be used.
type errInvalidPolicy struct {
	err error
}

// Error implements the error interface.
func (e errInvalidPolicy) Error() string {
	return e.err.Error()
}

var errNoTagsInDatabase = errors.New("no tags in database")

// imagePolicyOwnedConditions is a list of conditions owned by the
// ImagePolicyReconciler.
var imagePolicyOwnedConditions = []string{
	meta.ReadyCondition,
	meta.ReconcilingCondition,
	meta.StalledCondition,
}

// this is used as the key for the index of policy->repository; the
// string is arbitrary and acts as a reminder where the value comes
// from.
const imageRepoKey = ".spec.imageRepository"

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create

// ImagePolicyReconciler reconciles a ImagePolicy object
type ImagePolicyReconciler struct {
	client.Client
	kuberecorder.EventRecorder
	helper.Metrics

	ControllerName            string
	Database                  DatabaseReader
	ACLOptions                acl.Options
	AuthOptionsGetter         *registry.AuthOptionsGetter
	TokenCache                *cache.TokenCache
	DependencyRequeueInterval time.Duration

	patchOptions []patch.Option
}

type ImagePolicyReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

func (r *ImagePolicyReconciler) SetupWithManager(mgr ctrl.Manager, opts ImagePolicyReconcilerOptions) error {
	r.patchOptions = getPatchOptions(imagePolicyOwnedConditions, r.ControllerName)

	// index the policies by which image repo they point at, so that
	// it's easy to list those out when an image repo changes.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &imagev1.ImagePolicy{}, imageRepoKey, func(obj client.Object) []string {
		pol := obj.(*imagev1.ImagePolicy)

		namespace := pol.Spec.ImageRepositoryRef.Namespace
		if namespace == "" {
			namespace = obj.GetNamespace()
		}
		namespacedName := types.NamespacedName{
			Name:      pol.Spec.ImageRepositoryRef.Name,
			Namespace: namespace,
		}
		return []string{namespacedName.String()}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&imagev1.ImagePolicy{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicates.ReconcileRequestedPredicate{}),
		)).
		Watches(
			&imagev1.ImageRepository{},
			handler.EnqueueRequestsFromMapFunc(r.imagePoliciesForRepository),
			builder.WithPredicates(imageRepositoryPredicate{}),
		).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).
		Complete(r)
}

// imageRepositoryPredicate is used for watching changes to
// ImageRepository objects that are referenced by ImagePolicy
// objects.
type imageRepositoryPredicate struct {
	predicate.Funcs
}

func (imageRepositoryPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	newRepo := e.ObjectNew.(*imagev1.ImageRepository)
	if newRepo.Status.LastScanResult == nil {
		return false
	}

	oldRepo := e.ObjectOld.(*imagev1.ImageRepository)
	if oldRepo.Status.LastScanResult == nil ||
		oldRepo.Status.LastScanResult.Revision != newRepo.Status.LastScanResult.Revision {
		return true
	}

	return false
}

func (r *ImagePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	start := time.Now()
	log := ctrl.LoggerFrom(ctx)

	// Fetch the ImagePolicy.
	obj := &imagev1.ImagePolicy{}
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

		// Create patch options for patching the object.
		patchOpts := pkgreconcile.AddPatchOptions(obj, r.patchOptions, imagePolicyOwnedConditions, r.ControllerName)
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
	result, retErr = r.reconcile(ctx, serialPatcher, obj)
	return
}

// composeImagePolicyReadyMessage composes a Ready message for an ImagePolicy
// based on the results of applying the policy.
func composeImagePolicyReadyMessage(obj *imagev1.ImagePolicy) string {
	latestRef := obj.Status.LatestRef
	readyMsg := fmt.Sprintf("Latest image tag for %s resolved to %s", latestRef.Name, latestRef.Tag)
	if latestRef.Digest != "" {
		readyMsg += fmt.Sprintf(" with digest %s", latestRef.Digest)
	}
	if prev := obj.Status.ObservedPreviousRef; prev != nil && *latestRef != *prev {
		readyMsg += fmt.Sprintf(" (previously %s:%s", prev.Name, prev.Tag)
		if prev.Digest != "" {
			readyMsg += fmt.Sprintf("@%s", prev.Digest)
		}
		readyMsg += ")"
	}
	return readyMsg
}

func (r *ImagePolicyReconciler) reconcile(ctx context.Context, sp *patch.SerialPatcher, obj *imagev1.ImagePolicy) (result ctrl.Result, retErr error) {
	oldObj := obj.DeepCopy()

	// Set a default next reconcile time before processing the object.
	nextReconcileTime := obj.GetInterval()

	// If there's no error and no requeue is requested, it's a success.
	isSuccess := func(res ctrl.Result, err error) bool {
		if err != nil || res.RequeueAfter != nextReconcileTime {
			return false
		}
		return true
	}

	var readyMsg string
	defer func() {
		rs := pkgreconcile.NewResultFinalizer(isSuccess, readyMsg)
		retErr = rs.Finalize(obj, result, retErr)

		// Presence of reconciling means that the reconciliation didn't succeed.
		// Set the Reconciling reason to ProgressingWithRetry to indicate a
		// failure retry.
		if conditions.IsReconciling(obj) {
			reconciling := conditions.Get(obj, meta.ReconcilingCondition)
			reconciling.Reason = meta.ProgressingWithRetryReason
			conditions.Set(obj, reconciling)
		}

		notify(ctx, r.EventRecorder, oldObj, obj, readyMsg)
	}()

	// Validate errors in the spec before proceeding.
	if obj.GetDigestReflectionPolicy() == imagev1.ReflectAlways && obj.Spec.Interval == nil {
		const msg = "spec.interval must be set when spec.digestReflectionPolicy is set to 'Always'"
		conditions.MarkStalled(obj, imagev1.IntervalNotConfiguredReason, msg)
		result, retErr = ctrl.Result{}, nil
		return
	}

	// Set reconciling condition.
	pkgreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason, "reconciliation in progress")

	// Persist reconciling if generation differs.
	if obj.Generation != obj.Status.ObservedGeneration {
		pkgreconcile.ProgressiveStatus(false, obj, meta.ProgressingReason,
			"processing object: new generation %d -> %d", obj.Status.ObservedGeneration, obj.Generation)
		if err := sp.Patch(ctx, obj, r.patchOptions...); err != nil {
			result, retErr = ctrl.Result{}, err
			return
		}
	}

	// Get ImageRepository from reference.
	repo, err := r.getImageRepository(ctx, obj)
	if err != nil {
		reason := metav1.StatusFailure
		if _, ok := err.(errAccessDenied); ok {
			reason = aclapi.AccessDeniedReason
		}

		if apierrors.IsNotFound(err) {
			reason = imagev1.DependencyNotReadyReason
		}

		// Mark not ready and return a runtime error to retry. We need to retry
		// here because the access may be allowed due to change in objects not
		// watched by this reconciler, like the namespace that ImageRepository
		// allows access from.
		e := fmt.Errorf("failed to get the referred ImageRepository: %w", err)
		conditions.MarkFalse(obj, meta.ReadyCondition, reason, "%s", e)
		result, retErr = ctrl.Result{}, e
		return
	}

	// Check if the image is valid and mark stalled if not.
	if _, err := registry.ParseImageReference(repo.Spec.Image, repo.Spec.Insecure); err != nil {
		conditions.MarkStalled(obj, imagev1.ImageURLInvalidReason, "%s", err)
		result, retErr = ctrl.Result{}, nil
		return
	}

	// Check object-level workload identity feature gate.
	if repo.Spec.Provider != "generic" && repo.Spec.ServiceAccountName != "" && !auth.IsObjectLevelWorkloadIdentityEnabled() {
		const msgFmt = "to use spec.serviceAccountName in the ImageRepository for provider authentication please enable the %s feature gate in the controller"
		conditions.MarkStalled(obj, meta.FeatureGateDisabledReason, msgFmt,
			auth.FeatureGateObjectLevelWorkloadIdentity)
		result, retErr = ctrl.Result{}, nil
		return
	}

	// Construct a policer from the spec.policy.
	// Read the tags from database and use the policy to obtain a result for the
	// latest tag.
	latest, err := r.applyPolicy(obj, repo)
	if err != nil {
		// Stall if it's an invalid policy.
		if _, ok := err.(errInvalidPolicy); ok {
			conditions.MarkStalled(obj, "InvalidPolicy", "%s", err)
			result, retErr = ctrl.Result{}, nil
			return
		}

		// If there's no tag in the database, mark not ready and
		// requeue according to --requeue-dependency flag.
		if errors.Is(err, errNoTagsInDatabase) {
			depsErr := fmt.Errorf("retrying in %s error: %w", r.DependencyRequeueInterval.Round(time.Second), err)
			conditions.MarkFalse(obj, meta.ReadyCondition, imagev1.DependencyNotReadyReason, "%s", depsErr)
			result, retErr = ctrl.Result{RequeueAfter: r.DependencyRequeueInterval}, nil
			return
		}

		conditions.MarkFalse(obj, meta.ReadyCondition, metav1.StatusFailure, "%s", err)
		result, retErr = ctrl.Result{}, err
		return
	}

	// Update status fields with the latest tag and digest.
	if err := r.updateImageRefs(ctx, repo, obj, latest); err != nil {
		result, retErr = ctrl.Result{}, err
		return
	}

	// Compute ready message.
	readyMsg = composeImagePolicyReadyMessage(obj)

	// Let result finalizer compute the Ready condition.
	conditions.Delete(obj, meta.ReadyCondition)

	// Set the next reconcile time in the result based on the interval.
	result, retErr = ctrl.Result{RequeueAfter: nextReconcileTime}, nil
	return
}

// updateImageRefs updates the status fields of the ImagePolicy with the
// latest image and digest. It takes the digest reflection policy into
// account and fetches the digest if needed.
func (r *ImagePolicyReconciler) updateImageRefs(ctx context.Context,
	repo *imagev1.ImageRepository, obj *imagev1.ImagePolicy, latest string) error {

	latestRef := &imagev1.ImageRef{
		Name: repo.Spec.Image,
		Tag:  latest,
	}

	// Determine if we need to fetch the digest based on the reflection policy.
	var shouldFetch bool
	switch obj.GetDigestReflectionPolicy() {
	case imagev1.ReflectIfNotPresent:

		shouldFetch = obj.Status.LatestRef == nil ||
			obj.Status.LatestRef.Name != latestRef.Name ||
			obj.Status.LatestRef.Tag != latestRef.Tag ||
			obj.Status.LatestRef.Digest == ""

		if !shouldFetch {
			latestRef.Digest = obj.Status.LatestRef.Digest
		}

	case imagev1.ReflectAlways:
		shouldFetch = true
	}

	// Fetch the digest if needed.
	if shouldFetch {
		digest, err := r.fetchDigest(ctx, repo, obj, latest)
		if err != nil {
			return fmt.Errorf("failed fetching digest of %s: %w", latestRef.String(), err)
		}
		latestRef.Digest = digest
	}

	// Update the status fields only if the resulting ref is different.
	if obj.Status.LatestRef == nil || *latestRef != *obj.Status.LatestRef {
		obj.Status.ObservedPreviousRef = obj.Status.LatestRef
		obj.Status.LatestRef = latestRef
	}

	return nil
}

// fetchDigest fetches the digest of the given image repository and latest tag.
func (r *ImagePolicyReconciler) fetchDigest(ctx context.Context,
	repo *imagev1.ImageRepository, obj *imagev1.ImagePolicy, latest string) (string, error) {

	ref := repo.Spec.Image + ":" + latest
	tagRef, err := name.ParseReference(ref)
	if err != nil {
		return "", fmt.Errorf("failed parsing reference %q: %w", ref, err)
	}

	involvedObject := &cache.InvolvedObject{
		Kind:      imagev1.ImagePolicyKind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Operation: cache.OperationReconcile,
	}
	opts, err := r.AuthOptionsGetter.GetOptions(ctx, repo, involvedObject)
	if err != nil {
		return "", fmt.Errorf("failed to configure authentication options: %w", err)
	}

	desc, err := remote.Head(tagRef, opts...)
	if err != nil {
		return "", fmt.Errorf("failed fetching descriptor for %q: %w", tagRef.String(), err)
	}

	return desc.Digest.String(), nil
}

// getImageRepository tries to fetch an ImageRepository referenced by the given
// ImagePolicy if it's accessible.
func (r *ImagePolicyReconciler) getImageRepository(ctx context.Context, obj *imagev1.ImagePolicy) (*imagev1.ImageRepository, error) {
	repo := &imagev1.ImageRepository{}
	repoNamespacedName := types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Spec.ImageRepositoryRef.Name,
	}
	if obj.Spec.ImageRepositoryRef.Namespace != "" {
		repoNamespacedName.Namespace = obj.Spec.ImageRepositoryRef.Namespace
	}

	// If NoCrossNamespaceRefs is true and ImageRepository and ImagePolicy are
	// in different namespaces, the ImageRepository can't be accessed.
	if r.ACLOptions.NoCrossNamespaceRefs && repoNamespacedName.Namespace != obj.GetNamespace() {
		return nil, errAccessDenied{
			err: fmt.Errorf("cannot access '%s/%s', cross-namespace references have been blocked", imagev1.ImageRepositoryKind, repoNamespacedName),
		}
	}

	// Get the ImageRepository.
	if err := r.Get(ctx, repoNamespacedName, repo); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, fmt.Errorf("referenced %s does not exist: %w", imagev1.ImageRepositoryKind, err)
		}
	}

	// Check if the ImageRepository allows access to ImagePolicy.
	aclAuth := acl.NewAuthorization(r.Client)
	if err := aclAuth.HasAccessToRef(ctx, obj, repoNamespacedName, repo.Spec.AccessFrom); err != nil {
		return nil, errAccessDenied{err: fmt.Errorf("access denied: %w", err)}
	}

	return repo, nil
}

// applyPolicy reads the tags of the given repository from the internal database
// and applies the tag filters and constraints to return the latest image.
func (r *ImagePolicyReconciler) applyPolicy(obj *imagev1.ImagePolicy, repo *imagev1.ImageRepository) (string, error) {
	policer, err := policy.PolicerFromSpec(obj.Spec.Policy)
	if err != nil {
		return "", errInvalidPolicy{err: fmt.Errorf("invalid policy: %w", err)}
	}

	// Read tags from database with a maximum of 3 retries.
	tags, err := r.listTagsWithBackoff(repo.Status.CanonicalImageName)
	if err != nil {
		return "", err
	}

	// Apply tag filter.
	if obj.Spec.FilterTags != nil {
		filter, err := policy.NewRegexFilter(obj.Spec.FilterTags.Pattern, obj.Spec.FilterTags.Extract)
		if err != nil {
			return "", errInvalidPolicy{err: fmt.Errorf("failed to filter tags: %w", err)}
		}
		filter.Apply(tags)
		tags = filter.Items()
		latest, err := policer.Latest(tags)
		if err != nil {
			return "", err
		}
		return filter.GetOriginalTag(latest), nil
	}
	// Compute and return result.
	return policer.Latest(tags)
}

// reconcileDelete handles the deletion of the object.
func (r *ImagePolicyReconciler) reconcileDelete(obj *imagev1.ImagePolicy) (reconcile.Result, error) {
	// Remove our finalizer from the list.
	controllerutil.RemoveFinalizer(obj, imagev1.ImageFinalizer)

	// Cleanup caches.
	r.TokenCache.DeleteEventsForObject(imagev1.ImagePolicyKind,
		obj.GetName(), obj.GetNamespace(), cache.OperationReconcile)

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

func (r *ImagePolicyReconciler) imagePoliciesForRepository(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)
	var policies imagev1.ImagePolicyList
	if err := r.List(ctx, &policies, client.MatchingFields{imageRepoKey: client.ObjectKeyFromObject(obj).String()}); err != nil {
		log.Error(err, "failed to list ImagePolcies while getting reconcile requests for the same")
		return nil
	}
	reqs := make([]reconcile.Request, len(policies.Items))
	for i := range policies.Items {
		reqs[i].NamespacedName.Name = policies.Items[i].GetName()
		reqs[i].NamespacedName.Namespace = policies.Items[i].GetNamespace()
	}
	return reqs
}

// listTagsWithBackoff lists the tags of the given image from the
// internal database with retries if there are no tags in the database.
func (r *ImagePolicyReconciler) listTagsWithBackoff(canonicalImageName string) ([]string, error) {
	var backoff = wait.Backoff{
		Steps:    4,
		Duration: 1 * time.Second,
		Factor:   1.5,
		Jitter:   0.1,
	}

	var tags []string

	err := retry.OnError(backoff, func(err error) bool {
		return errors.Is(err, errNoTagsInDatabase)
	}, func() error {
		var err error
		tags, err = r.Database.Tags(canonicalImageName)
		if err != nil {
			return fmt.Errorf("failed to read tags from database: %w", err)
		}
		if len(tags) == 0 {
			return errNoTagsInDatabase
		}
		return nil
	})

	return tags, err
}
