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
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	aclapi "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/conditions"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	pkgreconcile "github.com/fluxcd/pkg/runtime/reconcile"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
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

// imagePolicyNegativeConditions is a list of negative polarity conditions
// owned by ImagePolicyReconciler. It is used in tests for compliance with
// kstatus.
var imagePolicyNegativeConditions = []string{
	meta.StalledCondition,
	meta.ReconcilingCondition,
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

// ImagePolicyReconciler reconciles a ImagePolicy object
type ImagePolicyReconciler struct {
	client.Client
	kuberecorder.EventRecorder
	helper.Metrics

	ControllerName    string
	Database          DatabaseReader
	ACLOptions        acl.Options
	AuthOptionsGetter registry.AuthOptionsGetter

	patchOptions []patch.Option
}

type ImagePolicyReconcilerOptions struct {
	RateLimiter ratelimiter.RateLimiter
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
		For(&imagev1.ImagePolicy{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&imagev1.ImageRepository{},
			handler.EnqueueRequestsFromMapFunc(r.imagePoliciesForRepository),
		).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).
		Complete(r)
}

func (r *ImagePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	start := time.Now()

	// Fetch the ImagePolicy.
	obj := &imagev1.ImagePolicy{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the patch helper with the current version of the object.
	serialPatcher := patch.NewSerialPatcher(obj, r.Client)

	// Always attempt to patch the object after each reconciliation.
	defer func() {
		// Create patch options for patching the object.
		patchOpts := pkgreconcile.AddPatchOptions(obj, r.patchOptions, imagePolicyOwnedConditions, r.ControllerName)
		if err := serialPatcher.Patch(ctx, obj, patchOpts...); err != nil {
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

	// Examine if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, obj)
	}

	// Add finalizer first if it doesn't exist to avoid the race condition
	// between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp
	// is not set.
	if !controllerutil.ContainsFinalizer(obj, imagev1.ImageFinalizer) {
		controllerutil.AddFinalizer(obj, imagev1.ImageFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	// Call subreconciler.
	result, retErr = r.reconcile(ctx, serialPatcher, obj)
	return
}

// composeImagePolicyReadyMessage composes a Ready message for an ImagePolicy
// based on the results of applying the policy.
func composeImagePolicyReadyMessage(previousTag, latestTag, image string) string {
	readyMsg := fmt.Sprintf("Latest image tag for '%s' resolved to %s", image, latestTag)
	if previousTag != "" && previousTag != latestTag {
		readyMsg = fmt.Sprintf("Latest image tag for '%s' updated from %s to %s", image, previousTag, latestTag)
	}
	return readyMsg
}

func (r *ImagePolicyReconciler) reconcile(ctx context.Context, sp *patch.SerialPatcher, obj *imagev1.ImagePolicy) (result ctrl.Result, retErr error) {
	oldObj := obj.DeepCopy()

	var resultImage, resultTag, previousTag string

	// If there's no error and no requeue is requested, it's a success.
	isSuccess := func(res ctrl.Result, err error) bool {
		if err != nil || res.Requeue {
			return false
		}
		return true
	}

	defer func() {
		readyMsg := composeImagePolicyReadyMessage(previousTag, resultTag, resultImage)

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

	// Cleanup the last result.
	obj.Status.LatestImage = ""
	obj.Status.LatestRef = nil

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
		conditions.MarkFalse(obj, meta.ReadyCondition, reason, e.Error())
		result, retErr = ctrl.Result{}, e
		return
	}

	// Proceed only if the ImageRepository has scan result.
	if repo.Status.LastScanResult == nil {
		// Mark not ready but don't requeue. When the repository becomes ready,
		// it'll trigger a policy reconciliation. No runtime error to prevent
		// requeue.
		conditions.MarkFalse(obj, meta.ReadyCondition, imagev1.DependencyNotReadyReason, "referenced ImageRepository has not been scanned yet")
		result, retErr = ctrl.Result{}, nil
		return
	}

	// Construct a policer from the spec.policy.
	// Read the tags from database and use the policy to obtain a result for the
	// latest tag.
	latest, err := r.applyPolicy(ctx, obj, repo)
	if err != nil {
		// Stall if it's an invalid policy.
		if _, ok := err.(errInvalidPolicy); ok {
			conditions.MarkStalled(obj, "InvalidPolicy", err.Error())
			result, retErr = ctrl.Result{}, nil
			return
		}

		// If there's no tag in the database, mark not ready and retry.
		if err == errNoTagsInDatabase {
			conditions.MarkFalse(obj, meta.ReadyCondition, imagev1.DependencyNotReadyReason, err.Error())
			result, retErr = ctrl.Result{}, err
			return
		}

		conditions.MarkFalse(obj, meta.ReadyCondition, metav1.StatusFailure, err.Error())
		result, retErr = ctrl.Result{}, err
		return
	}

	// Write the observations on status.
	obj.Status.LatestImage = repo.Spec.Image + ":" + latest
	lr := imagev1.ImageRef{Name: repo.Spec.Image, Tag: latest}
	obj.Status.LatestRef = &lr

	if err := r.updateDigest(ctx, repo, obj, oldObj, latest); err != nil {
		result, retErr = ctrl.Result{}, err
		return
	}

	// If the old latest image and new latest image don't match, set the old
	// image as the observed previous image.
	// NOTE: The following allows the previous image to be set empty when
	// there's a failure and a subsequent recovery from it. This behavior helps
	// avoid creating an update event as there's no previous image to infer
	// from. Recovery from a failure shouldn't result in an update event.
	if oldObj.Status.LatestRef != nil &&
		*oldObj.Status.LatestRef != *obj.Status.LatestRef {
		obj.Status.ObservedPreviousImage = oldObj.Status.LatestImage
		obj.Status.ObservedPreviousRef = oldObj.Status.LatestRef.DeepCopy()
	}

	// Parse the observed previous image if any and extract previous tag. This
	// is used to determine image tag update path.
	if obj.Status.ObservedPreviousRef != nil {
		img := obj.Status.ObservedPreviousRef.Name + ":" + obj.Status.ObservedPreviousRef.Tag
		prevRef, err := name.NewTag(img)
		if err != nil {
			e := fmt.Errorf("failed to parse previous image '%s': %w", img, err)
			conditions.MarkFalse(obj, meta.ReadyCondition, meta.FailedReason, e.Error())
			result, retErr = ctrl.Result{}, e
		}
		previousTag = prevRef.TagStr()
	}

	resultImage = repo.Spec.Image
	resultTag = latest

	conditions.Delete(obj, meta.ReadyCondition)

	result, retErr = ctrl.Result{}, nil
	return
}

func (r *ImagePolicyReconciler) updateDigest(ctx context.Context, repo *imagev1.ImageRepository, obj, oldObj *imagev1.ImagePolicy, tag string) error {
	if obj.GetDigestReflectionPolicy() == imagev1.ReflectNever {
		obj.Status.LatestRef.Digest = ""
		return nil
	}

	if obj.GetDigestReflectionPolicy() == imagev1.ReflectIfNotPresent &&
		oldObj.Status.LatestRef != nil &&
		oldObj.Status.LatestRef.Digest != "" &&
		obj.Status.LatestRef.Name == oldObj.Status.LatestRef.Name &&
		obj.Status.LatestRef.Tag == oldObj.Status.LatestRef.Tag {
		obj.Status.LatestRef.Digest = oldObj.Status.LatestRef.Digest
		return nil
	}

	var err error
	obj.Status.LatestRef.Digest, err = r.fetchDigest(ctx, repo, tag, obj)
	if err != nil {
		return fmt.Errorf("failed fetching digest of %s: %w", obj.Status.LatestRef.String(), err)
	}

	return nil
}

func (r *ImagePolicyReconciler) fetchDigest(ctx context.Context, repo *imagev1.ImageRepository, latest string, obj *imagev1.ImagePolicy) (string, error) {
	ref := strings.Join([]string{repo.Spec.Image, latest}, ":")
	tagRef, err := name.ParseReference(ref)
	if err != nil {
		return "", fmt.Errorf("failed parsing reference %q: %w", ref, err)
	}
	opts, err := r.AuthOptionsGetter(ctx, *repo)
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
func (r *ImagePolicyReconciler) applyPolicy(ctx context.Context, obj *imagev1.ImagePolicy, repo *imagev1.ImageRepository) (string, error) {
	policer, err := policy.PolicerFromSpec(obj.Spec.Policy)
	if err != nil {
		return "", errInvalidPolicy{err: fmt.Errorf("invalid policy: %w", err)}
	}

	// Read tags from database, apply and filter is configured and compute the
	// result.
	tags, err := r.Database.Tags(repo.Status.CanonicalImageName)
	if err != nil {
		return "", fmt.Errorf("failed to read tags from database: %w", err)
	}

	if len(tags) == 0 {
		return "", errNoTagsInDatabase
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
func (r *ImagePolicyReconciler) reconcileDelete(ctx context.Context, obj *imagev1.ImagePolicy) (reconcile.Result, error) {
	// Remove our finalizer from the list.
	controllerutil.RemoveFinalizer(obj, imagev1.ImageFinalizer)

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
