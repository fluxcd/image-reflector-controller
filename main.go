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

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dgraph-io/badger/v3"
	flag "github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"

	"github.com/fluxcd/pkg/oci/auth/login"
	"github.com/fluxcd/pkg/runtime/acl"
	"github.com/fluxcd/pkg/runtime/client"
	helper "github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/events"
	feathelper "github.com/fluxcd/pkg/runtime/features"
	"github.com/fluxcd/pkg/runtime/leaderelection"
	"github.com/fluxcd/pkg/runtime/logger"
	"github.com/fluxcd/pkg/runtime/metrics"
	"github.com/fluxcd/pkg/runtime/pprof"
	"github.com/fluxcd/pkg/runtime/probes"

	// +kubebuilder:scaffold:imports

	imagev1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	"github.com/fluxcd/image-reflector-controller/internal/controller"
	"github.com/fluxcd/image-reflector-controller/internal/database"
	"github.com/fluxcd/image-reflector-controller/internal/features"
)

const controllerName = "image-reflector-controller"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(imagev1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr             string
		eventsAddr              string
		healthAddr              string
		clientOptions           client.Options
		logOptions              logger.Options
		leaderElectionOptions   leaderelection.Options
		watchOptions            helper.WatchOptions
		connOptions             helper.ConnectionOptions
		storagePath             string
		storageValueLogFileSize int64
		concurrent              int
		awsAutoLogin            bool
		gcpAutoLogin            bool
		azureAutoLogin          bool
		aclOptions              acl.Options
		rateLimiterOptions      helper.RateLimiterOptions
		featureGates            feathelper.FeatureGates
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&eventsAddr, "events-addr", "", "The address of the events receiver.")
	flag.StringVar(&healthAddr, "health-addr", ":9440", "The address the health endpoint binds to.")
	flag.StringVar(&storagePath, "storage-path", "/data", "Where to store the persistent database of image metadata")
	flag.Int64Var(&storageValueLogFileSize, "storage-value-log-file-size", 1<<28, "Set the database's memory mapped value log file size in bytes. Effective memory usage is about two times this size.")
	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent resource reconciles.")

	// NOTE: Deprecated flags.
	flag.BoolVar(&awsAutoLogin, "aws-autologin-for-ecr", false, "(AWS) Attempt to get credentials for images in Elastic Container Registry, when no secret is referenced")
	flag.BoolVar(&gcpAutoLogin, "gcp-autologin-for-gcr", false, "(GCP) Attempt to get credentials for images in Google Container Registry, when no secret is referenced")
	flag.BoolVar(&azureAutoLogin, "azure-autologin-for-acr", false, "(Azure) Attempt to get credentials for images in Azure Container Registry, when no secret is referenced")

	clientOptions.BindFlags(flag.CommandLine)
	logOptions.BindFlags(flag.CommandLine)
	leaderElectionOptions.BindFlags(flag.CommandLine)
	aclOptions.BindFlags(flag.CommandLine)
	rateLimiterOptions.BindFlags(flag.CommandLine)
	featureGates.BindFlags(flag.CommandLine)
	watchOptions.BindFlags(flag.CommandLine)
	connOptions.BindFlags(flag.CommandLine)

	flag.Parse()

	logger.SetLogger(logger.NewLogger(logOptions))

	if err := connOptions.CheckEnvironmentCompatibility(); err != nil {
		setupLog.Error(err,
			"please verify that your controller flag settings are compatible with the controller's environment")
		os.Exit(1)
	}

	if awsAutoLogin || gcpAutoLogin || azureAutoLogin {
		setupLog.Error(errors.New("use of deprecated flags"),
			"autologin flags have been deprecated. These flags will be removed in a future release."+
				" Please update the respective ImageRepository objects with .spec.provider field.")
	}

	if err := featureGates.WithLogger(setupLog).SupportedFeatures(features.FeatureGates()); err != nil {
		setupLog.Error(err, "unable to load feature gates")
		os.Exit(1)
	}

	badgerOpts := badger.DefaultOptions(storagePath)
	badgerOpts.ValueLogFileSize = storageValueLogFileSize
	badgerDB, err := badger.Open(badgerOpts)
	if err != nil {
		setupLog.Error(err, "unable to open the Badger database")
		os.Exit(1)
	}
	defer badgerDB.Close()
	db := database.NewBadgerDatabase(badgerDB)

	watchNamespace := ""
	if !watchOptions.AllNamespaces {
		watchNamespace = os.Getenv("RUNTIME_NAMESPACE")
	}

	var disableCacheFor []ctrlclient.Object
	shouldCache, err := features.Enabled(features.CacheSecretsAndConfigMaps)
	if err != nil {
		setupLog.Error(err, "unable to check feature gate "+features.CacheSecretsAndConfigMaps)
		os.Exit(1)
	}
	if !shouldCache {
		disableCacheFor = append(disableCacheFor, &corev1.Secret{}, &corev1.ConfigMap{})
	}

	restConfig := client.GetConfigOrDie(clientOptions)

	watchSelector, err := helper.GetWatchSelector(watchOptions)
	if err != nil {
		setupLog.Error(err, "unable to configure watch label selector for manager")
		os.Exit(1)
	}

	leaderElectionID := fmt.Sprintf("%s-leader-election", controllerName)
	if watchOptions.LabelSelector != "" {
		leaderElectionID = leaderelection.GenerateID(leaderElectionID, watchOptions.LabelSelector)
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            metricsAddr,
		HealthProbeBindAddress:        healthAddr,
		LeaderElection:                leaderElectionOptions.Enable,
		LeaderElectionReleaseOnCancel: leaderElectionOptions.ReleaseOnCancel,
		LeaseDuration:                 &leaderElectionOptions.LeaseDuration,
		Logger:                        ctrl.Log,
		RenewDeadline:                 &leaderElectionOptions.RenewDeadline,
		RetryPeriod:                   &leaderElectionOptions.RetryPeriod,
		LeaderElectionID:              leaderElectionID,
		Controller: config.Controller{
			RecoverPanic:            pointer.Bool(true),
			MaxConcurrentReconciles: concurrent,
		},
		Client: ctrlclient.Options{
			Cache: &ctrlclient.CacheOptions{
				DisableFor: disableCacheFor,
			},
		},
		Cache: ctrlcache.Options{
			ByObject: map[ctrlclient.Object]ctrlcache.ByObject{
				&imagev1.ImageRepository{}: {Label: watchSelector},
				&imagev1.ImagePolicy{}:     {Label: watchSelector},
			},
			Namespaces: []string{watchNamespace},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	probes.SetupChecks(mgr, setupLog)
	pprof.SetupHandlers(mgr, setupLog)

	var eventRecorder *events.Recorder
	if eventRecorder, err = events.NewRecorder(mgr, ctrl.Log, eventsAddr, controllerName); err != nil {
		setupLog.Error(err, "unable to create event recorder")
		os.Exit(1)
	}

	metricsH := helper.NewMetrics(mgr, metrics.MustMakeRecorder(), imagev1.ImageFinalizer)

	if err := (&controller.ImageRepositoryReconciler{
		Client:         mgr.GetClient(),
		EventRecorder:  eventRecorder,
		Metrics:        metricsH,
		Database:       db,
		ControllerName: controllerName,
		DeprecatedLoginOpts: login.ProviderOptions{
			AwsAutoLogin:   awsAutoLogin,
			AzureAutoLogin: azureAutoLogin,
			GcpAutoLogin:   gcpAutoLogin,
		},
		AllowInsecureHTTP: connOptions.AllowHTTP,
	}).SetupWithManager(mgr, controller.ImageRepositoryReconcilerOptions{
		RateLimiter: helper.GetRateLimiter(rateLimiterOptions),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", imagev1.ImageRepositoryKind)
		os.Exit(1)
	}
	if err := (&controller.ImagePolicyReconciler{
		Client:         mgr.GetClient(),
		EventRecorder:  eventRecorder,
		Metrics:        metricsH,
		Database:       db,
		ACLOptions:     aclOptions,
		ControllerName: controllerName,
	}).SetupWithManager(mgr, controller.ImagePolicyReconcilerOptions{
		RateLimiter: helper.GetRateLimiter(rateLimiterOptions),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", imagev1.ImagePolicyKind)
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
