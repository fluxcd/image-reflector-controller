module github.com/fluxcd/image-reflector-controller

go 1.18

replace github.com/fluxcd/image-reflector-controller/api => ./api

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20221103172237-443f56ff4ba8
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/dgraph-io/badger/v3 v3.2103.4
	github.com/fluxcd/image-reflector-controller/api v0.23.0
	github.com/fluxcd/pkg/apis/acl v0.1.0
	github.com/fluxcd/pkg/apis/event v0.2.0
	github.com/fluxcd/pkg/apis/meta v0.18.0
	github.com/fluxcd/pkg/oci v0.15.0
	github.com/fluxcd/pkg/runtime v0.24.0
	github.com/fluxcd/pkg/version v0.2.0
	github.com/google/go-containerregistry v0.12.1
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20221114162634-781782aa2757
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.24.1
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.23.0
	k8s.io/api v0.25.4
	k8s.io/apimachinery v0.25.4
	k8s.io/client-go v0.25.4
	sigs.k8s.io/controller-runtime v0.13.1
)

require (
	cloud.google.com/go/compute v1.10.0 // indirect
	github.com/Azure/azure-sdk-for-go v67.0.0+incompatible // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.2.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.2.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.0.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.28 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.21 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v0.7.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/aws/aws-sdk-go v1.44.137 // indirect
	github.com/aws/aws-sdk-go-v2 v1.16.16 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.17.8 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.12.21 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.17.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.13.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.19 // indirect
	github.com/aws/smithy-go v1.13.3 // indirect
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20221004211355-a250ad2ca1e3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20221002210726-e883f69e0206 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.12.1 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/docker/cli v20.10.20+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.20+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/emicklei/go-restful/v3 v3.10.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/flatbuffers v2.0.7-0.20220318210805-d3aeee32bba0+incompatible // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20221017135236-9b4fdd506cdd // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.1 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/term v0.0.0-20221105221325-4eb28fa6025c // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc2 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/russross/blackfriday v1.6.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/cobra v1.6.1 // indirect
	github.com/vbatts/tar-split v0.11.2 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.starlark.net v0.0.0-20221028183056-acb66ad56dd2 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/crypto v0.1.0 // indirect
	golang.org/x/net v0.2.0 // indirect
	golang.org/x/oauth2 v0.2.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/term v0.2.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	golang.org/x/time v0.2.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.25.4 // indirect
	k8s.io/cli-runtime v0.25.4 // indirect
	k8s.io/component-base v0.25.4 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221110221610-a28e98eb7c70 // indirect
	k8s.io/kubectl v0.25.4 // indirect
	k8s.io/utils v0.0.0-20221108210102-8e77b1f39fe2 // indirect
	sigs.k8s.io/cli-utils v0.34.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kustomize/api v0.12.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.13.9 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
