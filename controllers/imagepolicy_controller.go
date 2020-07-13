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

	semver "github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	imagev1alpha1 "github.com/squaremo/image-update/api/v1alpha1"
)

type DatabaseReader interface {
	Tags(repo string) []string
}

// ImagePolicyReconciler reconciles a ImagePolicy object
type ImagePolicyReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Database DatabaseReader
}

// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.fluxcd.io,resources=imagepolicies/status,verbs=get;update;patch

func (r *ImagePolicyReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("imagepolicy", req.NamespacedName)

	var pol imagev1alpha1.ImagePolicy
	if err := r.Get(ctx, req.NamespacedName, &pol); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var repo imagev1alpha1.ImageRepository
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: pol.Namespace,
		Name:      pol.Spec.ImageRepository.Name,
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

	switch {
	case policy.SemVer != nil:
		latest, err := r.calculateLatestImageSemver(&policy, repo.Status.CanonicalImageName)
		if err != nil {
			return ctrl.Result{}, err
		}
		if latest != "" {
			pol.Status.LatestImage = repo.Spec.Image + ":" + latest
			err = r.Status().Update(ctx, &pol)
		}
		return ctrl.Result{}, err
	default:
		// no recognised policy, do nothing
	}

	return ctrl.Result{}, nil
}

func (r *ImagePolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&imagev1alpha1.ImagePolicy{}).
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
		if v, err := semver.NewVersion(tag); err == nil {
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
