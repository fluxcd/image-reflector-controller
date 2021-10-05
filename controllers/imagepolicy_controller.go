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

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	helpers "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/fluxcd/pkg/runtime/patch"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta1"
	"github.com/fluxcd/image-reflector-controller/internal/policy"
)

const (
	ImageRepositoryNotReadyReason = "ImageRepositoryNotReady"
	AccessDeniedReason            = "AccessDenied"
	ImagePolicyInvalidReason      = "InvalidPolicy"
)

const (
	EventReasonRefFound        = "ImageRefFound"
	EventReasonCannotCalculate = "CannotCalculate"
)

// this is used as the key for the index of policy->repository; the
// string is arbitrary and acts as a reminder where the value comes
// from.
const imageRepoKey = ".spec.imageRepository.name"

// ImagePolicyReconciler reconciles a ImagePolicy object
type ImagePolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Database DatabaseReader

	helpers.Events
	helpers.Metrics
}

type ImagePolicyReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ImagePolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	reconcileStart := time.Now()

	var pol imagev1.ImagePolicy
	if err := r.Get(ctx, req.NamespacedName, &pol); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := logr.FromContext(ctx)

	patcher, err := patch.NewHelper(&pol, r.Client)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	defer func() {
		if err := patcher.Patch(ctx, &pol, patch.WithOwnedConditions{
			Conditions: []string{meta.ReadyCondition},
		}, patch.WithStatusObservedGeneration{}); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		// Always record readiness and duration metrics
		r.RecordReadiness(ctx, &pol)
		r.RecordDuration(ctx, &pol, reconcileStart)
	}()

	var repo imagev1.ImageRepository
	repoNamespacedName := types.NamespacedName{
		Namespace: pol.Namespace,
		Name:      pol.Spec.ImageRepositoryRef.Name,
	}
	if pol.Spec.ImageRepositoryRef.Namespace != "" {
		repoNamespacedName.Namespace = pol.Spec.ImageRepositoryRef.Namespace
	}
	if err := r.Get(ctx, repoNamespacedName, &repo); err != nil {
		if client.IgnoreNotFound(err) == nil {
			conditions.MarkFalse(
				&pol,
				meta.ReadyCondition,
				ImageRepositoryNotReadyReason,
				err.Error(),
			)
			log.Error(err, "referenced ImageRepository does not exist")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// check if we are allowed to use the referenced ImageRepository
	if _, err := r.hasAccessToRepository(ctx, req, pol.Spec.ImageRepositoryRef, repo.Spec.AccessFrom); err != nil {
		conditions.MarkFalse(
			&pol,
			meta.ReadyCondition,
			AccessDeniedReason,
			err.Error(),
		)
		log.Error(err, "access denied")
		return ctrl.Result{}, nil
	}

	// if the image repo hasn't been scanned, don't bother
	if repo.Status.CanonicalImageName == "" {
		msg := "referenced ImageRepository has not been scanned yet"
		conditions.MarkFalse(
			&pol,
			meta.ReadyCondition,
			ImageRepositoryNotReadyReason,
			msg,
		)
		log.Info(msg)
		return ctrl.Result{}, nil
	}

	policer, err := policy.PolicerFromSpec(pol.Spec.Policy)
	if err != nil {
		msg := fmt.Sprintf("invalid policy: %s", err.Error())
		conditions.MarkFalse(
			&pol,
			meta.ReadyCondition,
			ImagePolicyInvalidReason,
			msg,
		)
		log.Error(err, "invalid policy")
		return ctrl.Result{}, nil
	}

	var latest string
	if policer != nil {
		var tags []string
		tags, err = r.Database.Tags(repo.Status.CanonicalImageName)
		if err == nil {
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

	if err != nil {
		conditions.MarkFalse(
			&pol,
			meta.ReadyCondition,
			meta.FailedReason,
			err.Error(),
		)
		r.Event(ctx, &pol, events.EventSeverityError, EventReasonCannotCalculate, err.Error())
		return ctrl.Result{}, err
	}

	if latest == "" {
		msg := fmt.Sprintf("Cannot determine latest tag for policy: %s", err.Error())
		pol.Status.LatestImage = ""
		conditions.MarkFalse(
			&pol,
			meta.ReadyCondition,
			meta.FailedReason,
			msg,
		)
		r.Event(ctx, &pol, events.EventSeverityError, EventReasonCannotCalculate, msg)
		return ctrl.Result{}, nil
	}

	msg := fmt.Sprintf("Latest image tag for '%s' resolved to: %s", repo.Spec.Image, latest)
	pol.Status.LatestImage = repo.Spec.Image + ":" + latest
	conditions.MarkTrue(
		&pol,
		meta.ReadyCondition,
		meta.SucceededReason,
		msg,
	)
	r.Event(ctx, &pol, events.EventSeverityInfo, EventReasonRefFound, msg)

	return ctrl.Result{}, err
}

func (r *ImagePolicyReconciler) SetupWithManager(mgr ctrl.Manager, opts ImagePolicyReconcilerOptions) error {
	// index the policies by which image repo they point at, so that
	// it's easy to list those out when an image repo changes.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &imagev1.ImagePolicy{}, imageRepoKey, func(obj client.Object) []string {
		pol := obj.(*imagev1.ImagePolicy)
		return []string{pol.Spec.ImageRepositoryRef.Name}
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
		}).
		Complete(r)
}

// ---

func (r *ImagePolicyReconciler) imagePoliciesForRepository(obj client.Object) []reconcile.Request {
	ctx := context.Background()
	var policies imagev1.ImagePolicyList
	if err := r.List(ctx, &policies, client.InNamespace(obj.GetNamespace()),
		client.MatchingFields{imageRepoKey: obj.GetName()}); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, len(policies.Items))
	for i := range policies.Items {
		reqs[i].NamespacedName.Name = policies.Items[i].GetName()
		reqs[i].NamespacedName.Namespace = policies.Items[i].GetNamespace()
	}
	return reqs
}

func (r *ImagePolicyReconciler) hasAccessToRepository(ctx context.Context, policy ctrl.Request, repo meta.NamespacedObjectReference, acl *imagev1.AccessFrom) (bool, error) {
	// grant access if the policy is in the same namespace as the repository
	if repo.Namespace == "" || policy.Namespace == repo.Namespace {
		return true, nil
	}

	// deny access if the repository has no ACL defined
	if acl == nil {
		return false, fmt.Errorf("ImageRepository '%s/%s' can't be accessed due to missing access list",
			repo.Namespace, repo.Name)
	}

	// get the policy namespace labels
	var policyNamespace v1.Namespace
	if err := r.Get(ctx, types.NamespacedName{Name: policy.Namespace}, &policyNamespace); err != nil {
		return false, err
	}
	policyLabels := policyNamespace.GetLabels()

	// check if the policy namespace labels match any ACL
	for _, selector := range acl.NamespaceSelectors {
		sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: selector.MatchLabels})
		if err != nil {
			return false, err
		}
		if sel.Matches(labels.Set(policyLabels)) {
			return true, nil
		}
	}

	return false, fmt.Errorf("ImageRepository '%s/%s' can't be accessed due to labels mismatch on namespace '%s'",
		repo.Namespace, repo.Name, policy.Namespace)
}
