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
	"fmt"
	"os"
	"time"

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
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/fluxcd/pkg/auth"
	pkgcache "github.com/fluxcd/pkg/cache"
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
	"github.com/fluxcd/image-reflector-controller/internal/registry"
)

const (
	controllerName = "image-reflector-controller"
	discardRatio   = 0.7
)

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
	const (
		tokenCacheDefaultMaxSize = 100
	)

	var (
		metricsAddr             string
		eventsAddr              string
		healthAddr              string
		clientOptions           client.Options
		logOptions              logger.Options
		leaderElectionOptions   leaderelection.Options
		watchOptions            helper.WatchOptions
		storagePath             string
		storageValueLogFileSize int64
		gcInterval              uint16 // max value is 65535 minutes (~ 45 days) which is well under the maximum time.Duration
		concurrent              int
		aclOptions              acl.Options
		rateLimiterOptions      helper.RateLimiterOptions
		featureGates            feathelper.FeatureGates
		tokenCacheOptions       pkgcache.TokenFlags
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&eventsAddr, "events-addr", "", "The address of the events receiver.")
	flag.StringVar(&healthAddr, "health-addr", ":9440", "The address the health endpoint binds to.")
	flag.StringVar(&storagePath, "storage-path", "/data", "Where to store the persistent database of image metadata")
	flag.Int64Var(&storageValueLogFileSize, "storage-value-log-file-size", 1<<28, "Set the database's memory mapped value log file size in bytes. Effective memory usage is about two times this size.")
	flag.Uint16Var(&gcInterval, "gc-interval", 10, "The number of minutes to wait between garbage collections. 0 disables the garbage collector.")
	flag.IntVar(&concurrent, "concurrent", 4, "The number of concurrent resource reconciles.")

	clientOptions.BindFlags(flag.CommandLine)
	logOptions.BindFlags(flag.CommandLine)
	leaderElectionOptions.BindFlags(flag.CommandLine)
	aclOptions.BindFlags(flag.CommandLine)
	rateLimiterOptions.BindFlags(flag.CommandLine)
	featureGates.BindFlags(flag.CommandLine)
	watchOptions.BindFlags(flag.CommandLine)
	tokenCacheOptions.BindFlags(flag.CommandLine, tokenCacheDefaultMaxSize)

	flag.Parse()

	logger.SetLogger(logger.NewLogger(logOptions))

	if err := featureGates.WithLogger(setupLog).SupportedFeatures(features.FeatureGates()); err != nil {
		setupLog.Error(err, "unable to load feature gates")
		os.Exit(1)
	}

	switch enabled, err := features.Enabled(auth.FeatureGateObjectLevelWorkloadIdentity); {
	case err != nil:
		setupLog.Error(err, "unable to check feature gate "+auth.FeatureGateObjectLevelWorkloadIdentity)
		os.Exit(1)
	case enabled:
		auth.EnableObjectLevelWorkloadIdentity()
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
	var badgerGC *database.BadgerGarbageCollector
	if gcInterval > 0 {
		badgerGC = database.NewBadgerGarbageCollector("badger-gc", badgerDB, time.Duration(gcInterval)*time.Minute, discardRatio)
	} else {
		setupLog.V(1).Info("Badger garbage collector is disabled")
	}

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

	mgrConfig := ctrl.Options{
		Scheme:                        scheme,
		HealthProbeBindAddress:        healthAddr,
		LeaderElection:                leaderElectionOptions.Enable,
		LeaderElectionReleaseOnCancel: leaderElectionOptions.ReleaseOnCancel,
		LeaseDuration:                 &leaderElectionOptions.LeaseDuration,
		Logger:                        ctrl.Log,
		RenewDeadline:                 &leaderElectionOptions.RenewDeadline,
		RetryPeriod:                   &leaderElectionOptions.RetryPeriod,
		LeaderElectionID:              leaderElectionID,
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
		},
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			ExtraHandlers: pprof.GetHandlers(),
		},
		Controller: config.Controller{
			RecoverPanic:            pointer.Bool(true),
			MaxConcurrentReconciles: concurrent,
		},
	}

	if watchNamespace != "" {
		mgrConfig.Cache.DefaultNamespaces = map[string]ctrlcache.Config{
			watchNamespace: ctrlcache.Config{},
		}
	}

	mgr, err := ctrl.NewManager(restConfig, mgrConfig)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if badgerGC != nil {
		mgr.Add(badgerGC)
	}

	probes.SetupChecks(mgr, setupLog)

	var eventRecorder *events.Recorder
	if eventRecorder, err = events.NewRecorder(mgr, ctrl.Log, eventsAddr, controllerName); err != nil {
		setupLog.Error(err, "unable to create event recorder")
		os.Exit(1)
	}

	metricsH := helper.NewMetrics(mgr, metrics.MustMakeRecorder(), imagev1.ImageFinalizer)

	var tokenCache *pkgcache.TokenCache
	if tokenCacheOptions.MaxSize > 0 {
		var err error
		tokenCache, err = pkgcache.NewTokenCache(tokenCacheOptions.MaxSize,
			pkgcache.WithMaxDuration(tokenCacheOptions.MaxDuration),
			pkgcache.WithMetricsRegisterer(ctrlmetrics.Registry),
			pkgcache.WithMetricsPrefix("gotk_token_"))
		if err != nil {
			setupLog.Error(err, "unable to create token cache")
			os.Exit(1)
		}
	}

	authOptionsGetter := &registry.AuthOptionsGetter{
		Client:     mgr.GetClient(),
		TokenCache: tokenCache,
	}

	if err := (&controller.ImageRepositoryReconciler{
		Client:            mgr.GetClient(),
		EventRecorder:     eventRecorder,
		Metrics:           metricsH,
		Database:          db,
		ControllerName:    controllerName,
		TokenCache:        tokenCache,
		AuthOptionsGetter: authOptionsGetter,
	}).SetupWithManager(mgr, controller.ImageRepositoryReconcilerOptions{
		RateLimiter: helper.GetRateLimiter(rateLimiterOptions),
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", imagev1.ImageRepositoryKind)
		os.Exit(1)
	}
	if err := (&controller.ImagePolicyReconciler{
		Client:            mgr.GetClient(),
		EventRecorder:     eventRecorder,
		Metrics:           metricsH,
		Database:          db,
		ACLOptions:        aclOptions,
		ControllerName:    controllerName,
		AuthOptionsGetter: authOptionsGetter,
		TokenCache:        tokenCache,
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
