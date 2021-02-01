module github.com/fluxcd/image-reflector-controller

go 1.15

replace github.com/fluxcd/image-reflector-controller/api => ./api

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/dgraph-io/badger/v3 v3.2011.1
	github.com/fluxcd/image-reflector-controller/api v0.5.0
	github.com/fluxcd/pkg/apis/meta v0.7.0
	github.com/fluxcd/pkg/runtime v0.8.0
	github.com/fluxcd/pkg/version v0.0.1
	github.com/go-logr/logr v0.3.0
	github.com/google/go-containerregistry v0.1.1
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.0
)
