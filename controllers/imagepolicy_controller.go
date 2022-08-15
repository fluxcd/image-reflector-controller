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

package controllers

import (
	"context"
	"fmt"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	aclapi "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/fluxcd/pkg/runtime/metrics"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	"github.com/fluxcd/image-reflector-controller/internal/policy"
)

// this is used as the key for the index of policy->repository; the
// string is arbitrary and acts as a reminder where the value comes
// from.
const imageRepoKey = ".spec.imageRepository"

// ImagePolicyReconciler reconciles a ImagePolicy object
type ImagePolicyReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	EventRecorder   kuberecorder.EventRecorder
	MetricsRecorder *metrics.Recorder
	Database        DatabaseReader
	ACLOptions      acl.Options
}

type ImagePolicyReconcilerOptions struct {
	MaxConcurrentReconciles int
	RateLimiter             ratelimiter.RateLimiter
}

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ImagePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcileStart := time.Now()

	var pol imagev1.ImagePolicy
	if err := r.Get(ctx, req.NamespacedName, &pol); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := ctrl.LoggerFrom(ctx)

	// record reconciliation duration
	if r.MetricsRecorder != nil {
		objRef, err := reference.GetReference(r.Scheme, &pol)
		if err != nil {
			return ctrl.Result{}, err
		}
		defer r.MetricsRecorder.RecordDuration(*objRef, reconcileStart)
	}
	defer r.recordReadinessMetric(ctx, &pol)

	// Add our finalizer if it does not exist.
	if !controllerutil.ContainsFinalizer(&pol, imagev1.ImagePolicyFinalizer) {
		patch := client.MergeFrom(pol.DeepCopy())
		controllerutil.AddFinalizer(&pol, imagev1.ImagePolicyFinalizer)
		if err := r.Patch(ctx, &pol, patch); err != nil {
			log.Error(err, "unable to register finalizer")
			return ctrl.Result{}, err
		}
	}

	// If the object is under deletion, record the readiness, and remove our finalizer.
	if !pol.ObjectMeta.DeletionTimestamp.IsZero() {
		r.recordReadinessMetric(ctx, &pol)
		controllerutil.RemoveFinalizer(&pol, imagev1.ImagePolicyFinalizer)
		if err := r.Update(ctx, &pol); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var repo imagev1.ImageRepository
	repoNamespacedName := types.NamespacedName{
		Namespace: pol.Namespace,
		Name:      pol.Spec.ImageRepositoryRef.Name,
	}
	if pol.Spec.ImageRepositoryRef.Namespace != "" {
		repoNamespacedName.Namespace = pol.Spec.ImageRepositoryRef.Namespace
	}

	recordError := func(err error, reason string) (ctrl.Result, error) {
		r.event(ctx, pol, events.EventSeverityError, err.Error())
		imagev1.SetImagePolicyReadiness(&pol, metav1.ConditionFalse, reason, err.Error())
		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			err = fmt.Errorf("failed to patch ImagePolicy: %s.%s status: %w", pol.GetName(), pol.GetNamespace(), err)
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{}, nil
	}
	recordErrorAndLog := func(err error, errorMsg, reason string) (ctrl.Result, error) {
		log.Error(err, errorMsg)
		return recordError(err, reason)
	}

	// check if we're allowed to reference across namespaces, before trying to fetch it
	if r.ACLOptions.NoCrossNamespaceRefs && repoNamespacedName.Namespace != pol.GetNamespace() {
		err := fmt.Errorf("cannot access '%s/%s', cross-namespace references have been blocked", imagev1.ImageRepositoryKind, repoNamespacedName)
		// this cannot proceed until the spec changes, so no need to requeue explicitly
		return recordErrorAndLog(err, "access denied to cross-namespace ImageRepository", aclapi.AccessDeniedReason)
	}

	if err := r.Get(ctx, repoNamespacedName, &repo); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return recordErrorAndLog(err, "referenced ImageRepository does not exist", imagev1.DependencyNotReadyReason)
		}
		return ctrl.Result{}, err
	}

	// check if we are allowed to use the referenced ImageRepository

	aclAuth := acl.NewAuthorization(r.Client)
	if err := aclAuth.HasAccessToRef(ctx, &pol, repoNamespacedName, repo.Spec.AccessFrom); err != nil {
		return recordErrorAndLog(err, "access denied", aclapi.AccessDeniedReason)
	}

	// if the image repo hasn't been scanned, don't bother
	if repo.Status.CanonicalImageName == "" {
		msg := "referenced ImageRepository has not been scanned yet"
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			imagev1.DependencyNotReadyReason,
			msg,
		)
		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Info(msg)
		return ctrl.Result{}, nil
	}

	policer, err := policy.PolicerFromSpec(pol.Spec.Policy)
	if err != nil {
		return recordErrorAndLog(err, "invalid policy", "InvalidPolicy")
	}

	var latest string
	if policer != nil {
		var tags []string
		tags, err = r.Database.Tags(repo.Status.CanonicalImageName)
		if err == nil {
			if len(tags) == 0 {
				msg := fmt.Sprintf("no tags found in local storage for '%s'", repo.Name)
				r.event(ctx, pol, events.EventSeverityInfo, msg)
				log.Info(msg)

				return ctrl.Result{}, nil
			}

			var filter *policy.RegexFilter
			if pol.Spec.FilterTags != nil {
				filter, err = policy.NewRegexFilter(pol.Spec.FilterTags.Pattern, pol.Spec.FilterTags.Extract)
				if err == nil {
					filter.Apply(tags)
					tags = filter.Items()
					latest, err = policer.Latest(tags)
					if err == nil {
						latest = filter.GetOriginalTag(latest)
					}
				}
			} else {
				latest, err = policer.Latest(tags)
			}
		}
	}

	if err != nil || latest == "" {
		pol.Status.LatestImage = ""
		if err == nil {
			err = fmt.Errorf("Cannot determine latest tag for policy")
		} else {
			err = fmt.Errorf("Cannot determine latest tag for policy: %w", err)
		}
		res, recErr := recordError(err, imagev1.ReconciliationFailedReason)
		if recErr != nil {
			// log the actual error since we are returning the error related to patching status
			log.Error(err, "")
			return res, recErr
		}
		return ctrl.Result{}, err
	}

	msg := fmt.Sprintf("Latest image tag for '%s' resolved to: %s", repo.Spec.Image, latest)
	pol.Status.LatestImage = repo.Spec.Image + ":" + latest
	imagev1.SetImagePolicyReadiness(
		&pol,
		metav1.ConditionTrue,
		imagev1.ReconciliationSucceededReason,
		msg,
	)

	if err := r.patchStatus(ctx, req, pol.Status); err != nil {
		return ctrl.Result{}, err
	}
	r.event(ctx, pol, events.EventSeverityInfo, msg)

	return ctrl.Result{}, err
}

func (r *ImagePolicyReconciler) SetupWithManager(mgr ctrl.Manager, opts ImagePolicyReconcilerOptions) error {
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
		For(&imagev1.ImagePolicy{}).
		Watches(
			&source.Kind{Type: &imagev1.ImageRepository{}},
			handler.EnqueueRequestsFromMapFunc(r.imagePoliciesForRepository),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: opts.MaxConcurrentReconciles,
			RateLimiter:             opts.RateLimiter,
			RecoverPanic:            true,
		}).
		Complete(r)
}

// ---

func (r *ImagePolicyReconciler) imagePoliciesForRepository(obj client.Object) []reconcile.Request {
	ctx := context.Background()
	var policies imagev1.ImagePolicyList
	if err := r.List(ctx, &policies, client.MatchingFields{imageRepoKey: client.ObjectKeyFromObject(obj).String()}); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, len(policies.Items))
	for i := range policies.Items {
		reqs[i].NamespacedName.Name = policies.Items[i].GetName()
		reqs[i].NamespacedName.Namespace = policies.Items[i].GetNamespace()
	}
	return reqs
}

// event emits a Kubernetes event and forwards the event to notification controller if configured
func (r *ImagePolicyReconciler) event(ctx context.Context, policy imagev1.ImagePolicy, severity, msg string) {
	eventtype := "Normal"
	if severity == events.EventSeverityError {
		eventtype = "Warning"
	}
	r.EventRecorder.Eventf(&policy, eventtype, severity, msg)
}

func (r *ImagePolicyReconciler) recordReadinessMetric(ctx context.Context, policy *imagev1.ImagePolicy) {
	if r.MetricsRecorder == nil {
		return
	}

	objRef, err := reference.GetReference(r.Scheme, policy)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "unable to record readiness metric")
		return
	}
	if rc := apimeta.FindStatusCondition(policy.Status.Conditions, meta.ReadyCondition); rc != nil {
		r.MetricsRecorder.RecordCondition(*objRef, *rc, !policy.DeletionTimestamp.IsZero())
	} else {
		r.MetricsRecorder.RecordCondition(*objRef, metav1.Condition{
			Type:   meta.ReadyCondition,
			Status: metav1.ConditionUnknown,
		}, !policy.DeletionTimestamp.IsZero())
	}
}

func (r *ImagePolicyReconciler) patchStatus(ctx context.Context, req ctrl.Request,
	newStatus imagev1.ImagePolicyStatus) error {
	var res imagev1.ImagePolicy
	if err := r.Get(ctx, req.NamespacedName, &res); err != nil {
		return err
	}

	patch := client.MergeFrom(res.DeepCopy())
	res.Status = newStatus

	return r.Status().Patch(ctx, &res, patch)
}
