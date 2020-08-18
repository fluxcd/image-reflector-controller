module github.com/fluxcd/image-reflector-controller

go 1.14

replace github.com/fluxcd/image-reflector-controller/api => ./api

require (
	github.com/Masterminds/semver/v3 v3.1.0
	github.com/fluxcd/image-reflector-controller/api v0.0.0-00010101000000-000000000000
	github.com/fluxcd/pkg/recorder v0.0.5
	github.com/go-logr/logr v0.1.0
	github.com/google/go-containerregistry v0.1.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	go.uber.org/zap v1.10.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
)
