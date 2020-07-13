/*
Copyright 2020 Michael Bridgen <mikeb@squaremobius.net>

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
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	imagev1alpha1 "github.com/squaremo/image-update/api/v1alpha1"
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
	Database DatabaseWriter
}

// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagerepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagerepositories/status,verbs=get;update;patch

func (r *ImageRepositoryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("imagerepository", req.NamespacedName)

	var imageRepo imagev1alpha1.ImageRepository
	if err := r.Get(ctx, req.NamespacedName, &imageRepo); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	ref, err := name.ParseReference(imageRepo.Spec.Image)
	if err != nil {
		imageRepo.Status.LastError = err.Error()
		if err := r.Status().Update(ctx, &imageRepo); err != nil {
			return ctrl.Result{}, err
		}
		log.Error(err, "Unable to parse image name", "imageName", imageRepo.Spec.Image)
		return ctrl.Result{}, nil
	}

	canonicalName := ref.Context().String()
	if imageRepo.Status.CanonicalImageName != canonicalName {
		imageRepo.Status.CanonicalImageName = canonicalName
	}

	now := time.Now()
	ok, when := shouldScan(&imageRepo, now)
	if ok {
		ctx, cancel := context.WithTimeout(ctx, scanTimeout)
		defer cancel()
		tags, err := remote.ListWithContext(ctx, ref.Context()) // TODO: auth
		if err != nil {
			imageRepo.Status.LastError = err.Error()
			if err = r.Status().Update(ctx, &imageRepo); err != nil {
				return ctrl.Result{}, err
			}
		}

		imageRepo.Status.LastScanTime = &metav1.Time{Time: now}
		imageRepo.Status.LastScanResult.TagCount = len(tags)
		imageRepo.Status.LastError = ""
		// share the information in the database
		r.Database.SetTags(canonicalName, tags)
		log.Info("successful scan", "tag count", len(tags))
		if err = r.Status().Update(ctx, &imageRepo); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: when}, nil
}

// shouldScan takes an image repo and the time now, and says whether
// the repository should be scanned now, and how long to wait for the
// next scan.
func shouldScan(repo *imagev1alpha1.ImageRepository, now time.Time) (bool, time.Duration) {
	scanInterval := defaultScanInterval
	if repo.Spec.ScanInterval != nil {
		scanInterval = repo.Spec.ScanInterval.Duration
	}

	if repo.Status.LastScanTime == nil {
		return true, scanInterval
	}
	when := scanInterval - now.Sub(repo.Status.LastScanTime.Time)
	if when < time.Second {
		return true, scanInterval
	}
	return false, when
}

func (r *ImageRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&imagev1alpha1.ImageRepository{}).
		Complete(r)
}
