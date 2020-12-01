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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/fluxcd/pkg/runtime/metrics"
	"github.com/fluxcd/pkg/runtime/predicates"

	imagev1alpha1 "github.com/fluxcd/image-reflector-controller/api/v1alpha1"
)

type DatabaseWriter interface {
	SetTags(repo string, tags []string)
}

// ImageRepositoryReconciler reconciles a ImageRepository object
type ImageRepositoryReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	EventRecorder         kuberecorder.EventRecorder
	ExternalEventRecorder *events.Recorder
	MetricsRecorder       *metrics.Recorder
	Database              interface {
		DatabaseWriter
		DatabaseReader
	}
}

// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.toolkit.fluxcd.io,resources=imagerepositories/status,verbs=get;update;patch

func (r *ImageRepositoryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	reconcileStart := time.Now()

	// NB: In general, if an error is returned then controller-runtime
	// will requeue the request with back-off. In the following this
	// is usually made explicit by _also_ returning
	// `ctrl.Result{Requeue: true}`.

	var imageRepo imagev1alpha1.ImageRepository
	if err := r.Get(ctx, req.NamespacedName, &imageRepo); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log := r.Log.WithValues("controller", strings.ToLower(imagev1alpha1.ImageRepositoryKind), "request", req.NamespacedName)

	// record rediness metric
	defer r.recordReadinessMetric(&imageRepo)

	if imageRepo.Spec.Suspend {
		msg := "ImageRepository is suspended, skipping reconciliation"
		imagev1alpha1.SetImageRepositoryReadiness(
			&imageRepo,
			metav1.ConditionFalse,
			meta.SuspendedReason,
			msg,
		)
		if err := r.Status().Update(ctx, &imageRepo); err != nil {
			log.Error(err, "unable to update status")
			return ctrl.Result{Requeue: true}, err
		}
		log.Info(msg)
		return ctrl.Result{}, nil
	}

	// record reconciliation duration
	if r.MetricsRecorder != nil {
		objRef, err := reference.GetReference(r.Scheme, &imageRepo)
		if err != nil {
			return ctrl.Result{}, err
		}
		defer r.MetricsRecorder.RecordDuration(*objRef, reconcileStart)
	}

	ref, err := name.ParseReference(imageRepo.Spec.Image)
	if err != nil {
		imagev1alpha1.SetImageRepositoryReadiness(
			&imageRepo,
			metav1.ConditionFalse,
			imagev1alpha1.ImageURLInvalidReason,
			err.Error(),
		)
		if err := r.Status().Update(ctx, &imageRepo); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		log.Error(err, "Unable to parse image name", "imageName", imageRepo.Spec.Image)
		return ctrl.Result{Requeue: true}, err
	}

	// Set CanonicalImageName based on the parsed reference
	if c := ref.Context().String(); imageRepo.Status.CanonicalImageName != c {
		imageRepo.Status.CanonicalImageName = c
		if err = r.Status().Update(ctx, &imageRepo); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	// Throttle scans based on spec Interval
	ok, when := r.shouldScan(imageRepo, reconcileStart)
	if ok {
		reconcileErr := r.scan(ctx, &imageRepo, ref)
		if err := r.Status().Update(ctx, &imageRepo); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		if reconcileErr != nil {
			r.event(imageRepo, events.EventSeverityError, reconcileErr.Error())
			return ctrl.Result{Requeue: true}, reconcileErr
		}
		// emit successful scan event
		if rc := apimeta.FindStatusCondition(imageRepo.Status.Conditions, meta.ReconciliationSucceededReason); rc != nil {
			r.event(imageRepo, events.EventSeverityInfo, rc.Message)
		}
	}

	log.Info(fmt.Sprintf("reconciliation finished in %s, next run in %s",
		time.Now().Sub(reconcileStart).String(),
		when.String(),
	))

	return ctrl.Result{RequeueAfter: when}, nil
}

func (r *ImageRepositoryReconciler) scan(ctx context.Context, imageRepo *imagev1alpha1.ImageRepository, ref name.Reference) error {
	timeout := imageRepo.GetTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var options []remote.Option
	if imageRepo.Spec.SecretRef != nil {
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: imageRepo.GetNamespace(),
			Name:      imageRepo.Spec.SecretRef.Name,
		}, &secret); err != nil {
			imagev1alpha1.SetImageRepositoryReadiness(
				imageRepo,
				metav1.ConditionFalse,
				meta.ReconciliationFailedReason,
				err.Error(),
			)
			return err
		}
		auth, err := authFromSecret(secret, ref.Context().RegistryStr())
		if err != nil {
			imagev1alpha1.SetImageRepositoryReadiness(
				imageRepo,
				metav1.ConditionFalse,
				meta.ReconciliationFailedReason,
				err.Error(),
			)
			return err
		}
		options = append(options, remote.WithAuth(auth))
	}

	tags, err := remote.ListWithContext(ctx, ref.Context(), options...)
	if err != nil {
		imagev1alpha1.SetImageRepositoryReadiness(
			imageRepo,
			metav1.ConditionFalse,
			meta.ReconciliationFailedReason,
			err.Error(),
		)
		return err
	}

	canonicalName := ref.Context().String()
	// TODO: add context and error handling to database ops
	r.Database.SetTags(canonicalName, tags)

	scanTime := metav1.Now()
	imageRepo.Status.LastScanResult = &imagev1alpha1.ScanResult{
		TagCount: len(tags),
		ScanTime: scanTime,
	}

	// if the reconcile request annotation was set, consider it
	// handled (NB it doesn't matter here if it was changed since last
	// time)
	if token, ok := meta.ReconcileAnnotationValue(imageRepo.GetAnnotations()); ok {
		imageRepo.Status.SetLastHandledReconcileRequest(token)
	}

	imagev1alpha1.SetImageRepositoryReadiness(
		imageRepo,
		metav1.ConditionTrue,
		meta.ReconciliationSucceededReason,
		fmt.Sprintf("successful scan, found %v tags", len(tags)),
	)

	return nil
}

// shouldScan takes an image repo and the time now, and says whether
// the repository should be scanned now, and how long to wait for the
// next scan.
func (r *ImageRepositoryReconciler) shouldScan(repo imagev1alpha1.ImageRepository, now time.Time) (bool, time.Duration) {
	scanInterval := repo.Spec.Interval.Duration

	// never scanned; do it now
	lastScanResult := repo.Status.LastScanResult
	if lastScanResult == nil {
		return true, scanInterval
	}
	lastScanTime := lastScanResult.ScanTime

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

	when := scanInterval - now.Sub(lastScanTime.Time)
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

// ---

// authFromSecret creates an Authenticator that can be given to the
// `remote` funcs, from a Kubernetes secret. If the secret doesn't
// have the right format or data, it returns an error.
func authFromSecret(secret corev1.Secret, registry string) (authn.Authenticator, error) {
	switch secret.Type {
	case "kubernetes.io/dockerconfigjson":
		var dockerconfig struct {
			Auths map[string]authn.AuthConfig
		}
		configData := secret.Data[".dockerconfigjson"]
		if err := json.NewDecoder(bytes.NewBuffer(configData)).Decode(&dockerconfig); err != nil {
			return nil, err
		}
		auth, ok := dockerconfig.Auths[registry]
		if !ok {
			return nil, fmt.Errorf("auth for %q not found in secret %v", registry, types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()})
		}
		return authn.FromConfig(auth), nil
	default:
		return nil, fmt.Errorf("unknown secret type %q", secret.Type)
	}
}

// event emits a Kubernetes event and forwards the event to notification controller if configured
func (r *ImageRepositoryReconciler) event(repo imagev1alpha1.ImageRepository, severity, msg string) {
	if r.EventRecorder != nil {
		r.EventRecorder.Eventf(&repo, "Normal", severity, msg)
	}
	if r.ExternalEventRecorder != nil {
		objRef, err := reference.GetReference(r.Scheme, &repo)
		if err != nil {
			r.Log.WithValues(
				"request",
				fmt.Sprintf("%s/%s", repo.GetNamespace(), repo.GetName()),
			).Error(err, "unable to send event")
			return
		}

		if err := r.ExternalEventRecorder.Eventf(*objRef, nil, severity, severity, msg); err != nil {
			r.Log.WithValues(
				"request",
				fmt.Sprintf("%s/%s", repo.GetNamespace(), repo.GetName()),
			).Error(err, "unable to send event")
			return
		}
	}
}

func (r *ImageRepositoryReconciler) recordReadinessMetric(repo *imagev1alpha1.ImageRepository) {
	if r.MetricsRecorder == nil {
		return
	}

	objRef, err := reference.GetReference(r.Scheme, repo)
	if err != nil {
		r.Log.WithValues(
			strings.ToLower(repo.Kind),
			fmt.Sprintf("%s/%s", repo.GetNamespace(), repo.GetName()),
		).Error(err, "unable to record readiness metric")
		return
	}
	if rc := apimeta.FindStatusCondition(repo.Status.Conditions, meta.ReadyCondition); rc != nil {
		r.MetricsRecorder.RecordCondition(*objRef, *rc, !repo.DeletionTimestamp.IsZero())
	} else {
		r.MetricsRecorder.RecordCondition(*objRef, metav1.Condition{
			Type:   meta.ReadyCondition,
			Status: metav1.ConditionUnknown,
		}, !repo.DeletionTimestamp.IsZero())
	}
}
