module github.com/fluxcd/image-reflector-controller

go 1.16

replace github.com/fluxcd/image-reflector-controller/api => ./api

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aws/aws-sdk-go v1.42.9
	github.com/dgraph-io/badger/v3 v3.2103.2
	github.com/fluxcd/image-reflector-controller/api v0.13.2
	github.com/fluxcd/pkg/apis/meta v0.10.0
	github.com/fluxcd/pkg/runtime v0.12.0
	github.com/fluxcd/pkg/version v0.1.0
	github.com/go-logr/logr v0.4.0
	github.com/google/go-containerregistry v0.7.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.10.2
)

// Fix CVE-2021-41190
replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2
