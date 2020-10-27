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

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/recorder"
	"github.com/fluxcd/pkg/runtime/predicates"

	imagev1alpha1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
)

const (
	scanTimeout         = 10 * time.Second
	defaultScanInterval = 10 * time.Minute
)

type DatabaseWriter interface {
	SetTags(repo string, tags []string)
}

// ImageRepositoryReconciler reconciles a ImageRepository object
type ImageRepositoryReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Database interface {
		DatabaseWriter
		DatabaseReader
	}
	EventRecorder         kuberecorder.EventRecorder
	ExternalEventRecorder *recorder.EventRecorder
}

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories/status,verbs=get;update;patch

func (r *ImageRepositoryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	// NB: In general, if an error is returned then controller-runtime
	// will requeue the request with back-off. In the following this
	// is usually made explicit by _also_ returning
	// `ctrl.Result{Requeue: true}`.

	var imageRepo imagev1alpha1.ImageRepository
	if err := r.Get(ctx, req.NamespacedName, &imageRepo); err != nil {
		// _Might_ get requeued
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("controller", strings.ToLower(imagev1alpha1.ImageRepositoryKind), "request", req.NamespacedName)

	if imageRepo.Spec.Suspend {
		msg := "ImageRepository is suspended, skipping reconciliation"
		status := imagev1alpha1.SetImageRepositoryReadiness(
			imageRepo,
			corev1.ConditionFalse,
			imagev1alpha1.SuspendedReason,
			msg,
		)
		if err := r.Status().Update(ctx, &status); err != nil {
			log.Error(err, "unable to update status")
			return ctrl.Result{Requeue: true}, err
		}
		log.Info(msg)
		return ctrl.Result{}, nil
	}

	ref, err := name.ParseReference(imageRepo.Spec.Image)
	if err != nil {
		status := imagev1alpha1.SetImageRepositoryReadiness(
			imageRepo,
			corev1.ConditionFalse,
			imagev1alpha1.ImageURLInvalidReason,
			err.Error(),
		)
		if err := r.Status().Update(ctx, &status); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Error(err, "Unable to parse image name", "imageName", imageRepo.Spec.Image)
		return ctrl.Result{Requeue: true}, err
	}

	imageRepo.Status.CanonicalImageName = ref.Context().String()

	now := time.Now()
	ok, when := r.shouldScan(imageRepo, now)
	if ok {
		ctx, cancel := context.WithTimeout(ctx, scanTimeout)
		defer cancel()

		reconciledRepo, reconcileErr := r.scan(ctx, imageRepo, ref)
		if err = r.Status().Update(ctx, &reconciledRepo); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if reconcileErr != nil {
			return ctrl.Result{Requeue: true}, reconcileErr
		} else {
			log.Info(fmt.Sprintf("reconciliation finished in %s, next run in %s",
				time.Now().Sub(now).String(),
				when),
			)
		}
	}

	return ctrl.Result{RequeueAfter: when}, nil
}

func (r *ImageRepositoryReconciler) scan(ctx context.Context, imageRepo imagev1alpha1.ImageRepository, ref name.Reference) (imagev1alpha1.ImageRepository, error) {
	canonicalName := ref.Context().String()

	// TODO: implement auth
	tags, err := remote.ListWithContext(ctx, ref.Context())
	if err != nil {
		return imagev1alpha1.SetImageRepositoryReadiness(
			imageRepo,
			corev1.ConditionFalse,
			imagev1alpha1.ReconciliationFailedReason,
			err.Error(),
		), err
	}

	// TODO: add context and error handling to database ops
	r.Database.SetTags(canonicalName, tags)

	imageRepo.Status.LastScanResult.TagCount = len(tags)

	// if the reconcile request annotation was set, consider it
	// handled (NB it doesn't matter here if it was changed since last
	// time)
	if token, ok := meta.ReconcileAnnotationValue(imageRepo.GetAnnotations()); ok {
		imageRepo.Status.SetLastHandledReconcileRequest(token)
	}

	return imagev1alpha1.SetImageRepositoryReadiness(
		imageRepo,
		corev1.ConditionTrue,
		imagev1alpha1.ReconciliationSucceededReason,
		fmt.Sprintf("successful scan, found %v tags", len(tags)),
	), nil
}

// shouldScan takes an image repo and the time now, and says whether
// the repository should be scanned now, and how long to wait for the
// next scan.
func (r *ImageRepositoryReconciler) shouldScan(repo imagev1alpha1.ImageRepository, now time.Time) (bool, time.Duration) {
	scanInterval := defaultScanInterval
	if repo.Spec.ScanInterval != nil {
		scanInterval = repo.Spec.ScanInterval.Duration
	}

	// never scanned; do it now
	lastTransitionTime := imagev1alpha1.GetLastTransitionTime(repo)
	if lastTransitionTime == nil {
		return true, scanInterval
	}

	// Is the controller seeing this because the reconcileAt
	// annotation was tweaked? Despite the name of the annotation, all
	// that matters is that it's different.
	if syncAt, ok := meta.ReconcileAnnotationValue(repo.GetAnnotations()); ok {
		if syncAt != repo.Status.GetLastHandledReconcileRequest() {
			return true, scanInterval
		}
	}

	// when recovering, it's possible that the resource has a last
	// scan time, but there's no records because the database has been
	// dropped and created again.

	// FIXME If the repo exists, has been
	// scanned, and doesn't have any tags, this will mean a scan every
	// time the resource comes up for reconciliation.
	if tags := r.Database.Tags(repo.Status.CanonicalImageName); len(tags) == 0 {
		return true, scanInterval
	}

	when := scanInterval - now.Sub(lastTransitionTime.Time)
	if when < time.Second {
		return true, scanInterval
	}
	return false, when
}

func (r *ImageRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&imagev1alpha1.ImageRepository{}).
		WithEventFilter(predicates.ChangePredicate{}).
		Complete(r)
}
