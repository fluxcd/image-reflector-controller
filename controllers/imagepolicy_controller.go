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
	"fmt"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/fluxcd/pkg/version"

	imagev1alpha1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
)

// this is used as the key for the index of policy->repository; the
// string is arbitrary and acts as a reminder where the value comes
// from.
const imageRepoKey = ".spec.imageRepository.name"

type DatabaseReader interface {
	Tags(repo string) []string
}

// ImagePolicyReconciler reconciles a ImagePolicy object
type ImagePolicyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	controller.Events
	controller.Metrics
	Database DatabaseReader
}

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch

func (r *ImagePolicyReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reconcileStart := time.Now()

	var pol imagev1alpha1.ImagePolicy
	if err := r.Get(ctx, req.NamespacedName, &pol); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("controller", strings.ToLower(imagev1alpha1.ImagePolicyKind), "request", req.NamespacedName)

	objRef, refErr := reference.GetReference(r.Scheme, &pol)
	if refErr != nil {
		return ctrl.Result{}, refErr
	}
	// the duration is recorded, but there's no readiness condition at present
	defer r.RecordDuration(objRef, reconcileStart)

	var repo imagev1alpha1.ImageRepository
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: pol.Namespace,
		Name:      pol.Spec.ImageRepositoryRef.Name,
	}, &repo); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Error(err, "referenced ImageRepository does not exist")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// if the image repo hasn't been scanned, don't bother
	if repo.Status.CanonicalImageName == "" {
		log.Info("referenced ImageRepository has not been scanned yet")
		return ctrl.Result{}, nil
	}

	policy := pol.Spec.Policy
	var latest string
	var err error
	switch {
	case policy.SemVer != nil:
		latest, err = r.calculateLatestImageSemver(&policy, repo.Status.CanonicalImageName)
	}
	if err != nil {
		r.Event(objRef, &pol, events.EventSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	if latest != "" {
		pol.Status.LatestImage = repo.Spec.Image + ":" + latest
		if err := r.Status().Update(ctx, &pol); err != nil {
			r.Event(objRef, &pol, events.EventSeverityError, err.Error())
			return ctrl.Result{}, err
		}
		r.Event(objRef, &pol, events.EventSeverityInfo, fmt.Sprintf("Latest image tag for '%s' resolved to: %s", repo.Spec.Image, latest))
	}

	return ctrl.Result{}, nil
}

func (r *ImagePolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// index the policies by which image repo they point at, so that
	// it's easy to list those out when an image repo changes.
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &imagev1alpha1.ImagePolicy{}, imageRepoKey, func(obj runtime.Object) []string {
		pol := obj.(*imagev1alpha1.ImagePolicy)
		return []string{pol.Spec.ImageRepositoryRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&imagev1alpha1.ImagePolicy{}).
		Watches(
			&source.Kind{Type: &imagev1alpha1.ImageRepository{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.imagePoliciesForRepository),
			}).
		Complete(r)
}

// ---

func (r *ImagePolicyReconciler) calculateLatestImageSemver(pol *imagev1alpha1.ImagePolicyChoice, canonImage string) (string, error) {
	tags := r.Database.Tags(canonImage)
	constraint, err := semver.NewConstraint(pol.SemVer.Range)
	if err != nil {
		// FIXME this'll get a stack trace in the log, but may not deserve it
		return "", err
	}
	var latestVersion *semver.Version
	for _, tag := range tags {
		if v, err := version.ParseVersion(tag); err == nil {
			if constraint.Check(v) && (latestVersion == nil || v.GreaterThan(latestVersion)) {
				latestVersion = v
			}
		}
	}
	if latestVersion != nil {
		return latestVersion.Original(), nil
	}
	return "", nil
}

func (r *ImagePolicyReconciler) imagePoliciesForRepository(obj handler.MapObject) []reconcile.Request {
	ctx := context.Background()
	var policies imagev1alpha1.ImagePolicyList
	if err := r.List(ctx, &policies, client.InNamespace(obj.Meta.GetNamespace()), client.MatchingFields{imageRepoKey: obj.Meta.GetName()}); err != nil {
		r.Log.Error(err, "failed to list ImagePolicy for ImageRepository")
		return nil
	}
	reqs := make([]reconcile.Request, len(policies.Items), len(policies.Items))
	for i := range policies.Items {
		reqs[i].NamespacedName.Name = policies.Items[i].GetName()
		reqs[i].NamespacedName.Namespace = policies.Items[i].GetNamespace()
	}
	return reqs
}
