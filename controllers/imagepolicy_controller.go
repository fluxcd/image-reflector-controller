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
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/fluxcd/pkg/runtime/metrics"

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/policy"
)

// this is used as the key for the index of policy->repository; the
// string is arbitrary and acts as a reminder where the value comes
// from.
const imageRepoKey = ".spec.imageRepository"

// ImagePolicyReconciler reconciles a ImagePolicy object
type ImagePolicyReconciler struct {
	client.Client
	Scheme                *runtime.Scheme
	EventRecorder         kuberecorder.EventRecorder
	ExternalEventRecorder *events.Recorder
	MetricsRecorder       *metrics.Recorder
	Database              DatabaseReader
}

type ImagePolicyReconcilerOptions struct {
	MaxConcurrentReconciles int
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

	log := logr.FromContext(ctx)

	// record reconciliation duration
	if r.MetricsRecorder != nil {
		objRef, err := reference.GetReference(r.Scheme, &pol)
		if err != nil {
			return ctrl.Result{}, err
		}
		defer r.MetricsRecorder.RecordDuration(*objRef, reconcileStart)
	}
	defer r.recordReadinessMetric(ctx, &pol)

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
			imagev1.SetImagePolicyReadiness(
				&pol,
				metav1.ConditionFalse,
				meta.DependencyNotReadyReason,
				err.Error(),
			)
			if err := r.patchStatus(ctx, req, pol.Status); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			log.Error(err, "referenced ImageRepository does not exist")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// check if we are allowed to use the referenced ImageRepository
	if _, err := r.hasAccessToRepository(ctx, req, pol.Spec.ImageRepositoryRef, repo.Spec.AccessFrom); err != nil {
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			"AccessDenied",
			err.Error(),
		)
		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Error(err, "access denied")
		return ctrl.Result{}, nil
	}

	// if the image repo hasn't been scanned, don't bother
	if repo.Status.CanonicalImageName == "" {
		msg := "referenced ImageRepository has not been scanned yet"
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			meta.DependencyNotReadyReason,
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
		msg := fmt.Sprintf("invalid policy: %s", err.Error())
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			"InvalidPolicy",
			msg,
		)
		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Error(err, "invalid policy")
		return ctrl.Result{}, nil
	}

	if pol.Spec.FilterTags == nil || pol.Spec.FilterTags.Discriminator == "" {
		return r.nonDiscriminated(ctx, req, policer, repo, pol)
	}else{
		return r.discriminated(ctx, req, policer, repo, pol)
	}
}

func (r *ImagePolicyReconciler) nonDiscriminated(ctx context.Context, req ctrl.Request, policer policy.Policer, repo imagev1.ImageRepository, pol imagev1.ImagePolicy) (ctrl.Result, error) {
	var latest string
	var err error

	pol.Status.Distribution = nil
	pol.Status.NbDistribution = 0

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
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			meta.ReconciliationFailedReason,
			err.Error(),
		)
		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		r.event(ctx, pol, events.EventSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	if latest == "" {
		msg := fmt.Sprintf("Cannot determine latest tag for policy: %s", err.Error())
		pol.Status.LatestImage = ""
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			meta.ReconciliationFailedReason,
			msg,
		)

		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			return ctrl.Result{}, err
		}
		r.event(ctx, pol, events.EventSeverityError, msg)
		return ctrl.Result{}, nil
	}

	msg := fmt.Sprintf("Latest image tag for '%s' resolved to: %s", repo.Spec.Image, latest)
	pol.Status.LatestImage = repo.Spec.Image + ":" + latest
	imagev1.SetImagePolicyReadiness(
		&pol,
		metav1.ConditionTrue,
		meta.ReconciliationSucceededReason,
		msg,
	)

	if err := r.patchStatus(ctx, req, pol.Status); err != nil {
		return ctrl.Result{}, err
	}
	r.event(ctx, pol, events.EventSeverityInfo, msg)

	return ctrl.Result{}, err
}

func (r *ImagePolicyReconciler) discriminated(ctx context.Context, req ctrl.Request, policer policy.Policer, repo imagev1.ImageRepository, pol imagev1.ImagePolicy) (ctrl.Result, error) {
	distribution := map[string]imagev1.ImageAndAttributes{}
	latest := ""
	var err error

	listAttributes := []string{ pol.Spec.FilterTags.Discriminator, pol.Spec.FilterTags.Extract }
	if len(pol.Spec.FilterTags.Attributes) > 0 {
		listAttributes = append(listAttributes, pol.Spec.FilterTags.Attributes...)
	}

	if policer != nil {
		var tags []string
		tags, err = r.Database.Tags(repo.Status.CanonicalImageName)
		if err == nil {
			var filter *policy.RegexExtractor
			filter, err = policy.NewRegexExtractor(pol.Spec.FilterTags.Pattern, listAttributes)
			if err == nil {
				filter.Apply(tags)
				reduced, err := filter.Reduce(pol.Spec.FilterTags.Discriminator, pol.Spec.FilterTags.Extract, policer)

				if err == nil {
					distribByExtracted := map[string]string{}
					var extractedList []string

					for k, v := range reduced {
						distribution[k] = imagev1.ImageAndAttributes{
							Image:      repo.Spec.Image,
							Tag:        v.Tag,
							Attributes: v.Attributes,
						}
						distribByExtracted[v.Extracted] = k
						extractedList = append(extractedList, v.Extracted)
					}

					newer, err := policer.Latest(extractedList)

					if err == nil {
						img := distribution[distribByExtracted[newer]]
						latest = img.Tag
					}
				}
			}
		}
	}


	if err != nil {
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			meta.ReconciliationFailedReason,
			err.Error(),
		)
		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		r.event(ctx, pol, events.EventSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	if len(distribution) == 0 || latest == "" {
		msg := fmt.Sprintf("Cannot determine latest tag for policy: %s", pol.Name)
		pol.Status.LatestImage = ""
		pol.Status.Distribution = nil
		pol.Status.NbDistribution = 0
		imagev1.SetImagePolicyReadiness(
			&pol,
			metav1.ConditionFalse,
			meta.ReconciliationFailedReason,
			msg,
		)

		if err := r.patchStatus(ctx, req, pol.Status); err != nil {
			return ctrl.Result{}, err
		}
		r.event(ctx, pol, events.EventSeverityError, msg)
		return ctrl.Result{}, nil
	}

	distribStr, _ := json.Marshal(distribution)
	msg := fmt.Sprintf("Distribution of image for '%s' resolved to: %s", repo.Spec.Image, string(distribStr))
	pol.Status.Distribution = distribution
	pol.Status.NbDistribution = len(distribution)
	pol.Status.LatestImage = repo.Spec.Image + ":" + latest
	imagev1.SetImagePolicyReadiness(
		&pol,
		metav1.ConditionTrue,
		meta.ReconciliationSucceededReason,
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
	if r.EventRecorder != nil {
		r.EventRecorder.Event(&policy, "Normal", severity, msg)
	}
	if r.ExternalEventRecorder != nil {
		objRef, err := reference.GetReference(r.Scheme, &policy)
		if err == nil {
			err = r.ExternalEventRecorder.Eventf(*objRef, nil, severity, severity, msg)
		}
		if err != nil {
			logr.FromContext(ctx).Error(err, "unable to send event")
			return
		}
	}
}

func (r *ImagePolicyReconciler) recordReadinessMetric(ctx context.Context, policy *imagev1.ImagePolicy) {
	if r.MetricsRecorder == nil {
		return
	}

	objRef, err := reference.GetReference(r.Scheme, policy)
	if err != nil {
		logr.FromContext(ctx).Error(err, "unable to record readiness metric")
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
