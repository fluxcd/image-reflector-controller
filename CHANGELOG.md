# Changelog

## 0.31.2

**Release date:** 2024-02-01

This prerelease comes with an update to the Kubernetes dependencies to
v1.28.6 and various other dependencies have been updated to their latest version
to patch upstream CVEs.

In addition, the controller is now built with Go 1.21.

Improvements:
- ci: Enable dependabot gomod updates
  [#495](https://github.com/fluxcd/image-reflector-controller/pull/495)
- Update Go to 1.21
  [#493](https://github.com/fluxcd/image-reflector-controller/pull/493)
- tests/int: Add separate resource cleanup step
  [#489](https://github.com/fluxcd/image-reflector-controller/pull/489)
- Various dependency updates
  [#501](https://github.com/fluxcd/image-reflector-controller/pull/501)
  [#499](https://github.com/fluxcd/image-reflector-controller/pull/499)
  [#498](https://github.com/fluxcd/image-reflector-controller/pull/498)
  [#496](https://github.com/fluxcd/image-reflector-controller/pull/496)
  [#494](https://github.com/fluxcd/image-reflector-controller/pull/494)
  [#492](https://github.com/fluxcd/image-reflector-controller/pull/492)
  [#490](https://github.com/fluxcd/image-reflector-controller/pull/490)
  [#484](https://github.com/fluxcd/image-reflector-controller/pull/484)
  [#483](https://github.com/fluxcd/image-reflector-controller/pull/483)

## 0.31.1

**Release date:** 2023-12-11

This prerelease comes with updates to AWS dependencies to fix an issue with ECR authentication.

In addition, the container base image was updated to Alpine 3.19.

Improvements:
- build: update Alpine to 3.19
  [#480](https://github.com/fluxcd/image-reflector-controller/pull/480)
- Update dependencies
  [#481](https://github.com/fluxcd/image-reflector-controller/pull/481)

## 0.31.0

**Release date:** 2023-12-08

This prerelease comes with support for insecure HTTP registries using the
new `.spec.insecure` field on `ImageRepository` objects. This field is
optional and defaults to `false`.

In addition, the Kubernetes dependencies have been updated to v1.28.4 in
combination with an update of the controller's dependencies.

Lastly, tiny improvements have been made to some of the error messages the
controller emits.

Improvements:
- Address miscellaneous issues throughout code base
  [#452](https://github.com/fluxcd/image-reflector-controller/pull/452)
- Update dependencies to Kubernetes v1.28
  [#471](https://github.com/fluxcd/image-reflector-controller/pull/471)
- imagerepo: add `.spec.insecure` to `ImageRepository`
  [#472](https://github.com/fluxcd/image-reflector-controller/pull/472)
- Various dependency updates
  [#453](https://github.com/fluxcd/image-reflector-controller/pull/453)
  [#454](https://github.com/fluxcd/image-reflector-controller/pull/454)
  [#455](https://github.com/fluxcd/image-reflector-controller/pull/455)
  [#459](https://github.com/fluxcd/image-reflector-controller/pull/459)
  [#460](https://github.com/fluxcd/image-reflector-controller/pull/460)
  [#477](https://github.com/fluxcd/image-reflector-controller/pull/477)

## 0.30.0

**Release date:** 2023-08-23

This prerelease adds support for Secrets of type 
[`kubernetes.io/tls`](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets) ImageRepositories'
`.spec.certSecretRef`. Note: Support for the `caFile`, `certFile` and `keyFile` keys has
been deprecated and will be removed in upcoming releases. After upgrading the controller to version 0.30.0, please
change all Secrets referenced in `.spec.certSecretRef` to follow the new format.

Starting with this version, the controller now stops exporting an object's metrics as soon as the object has been
deleted.

In addition, this version fixes handling of finalizers and updates the controller's dependencies.

Improvements:

- Update dependencies
  [#441](https://github.com/fluxcd/image-reflector-controller/pull/431)
- imagerepo: adopt Kubernetes style TLS secrets
  [#434](https://github.com/fluxcd/image-reflector-controller/pull/434)
- Delete stale metrics on object delete
  [#430](https://github.com/fluxcd/image-reflector-controller/pull/430)
- Update pkg/oci to support Azure China and US gov
  [438](https://github.com/fluxcd/image-reflector-controller/pull/438)

## 0.29.1

**Release date:** 2023-07-10

This is a patch release that fixes the AWS authentication for cross-region ECR repositories.

Fixes:
- Update `fluxcd/pkg/oci` to fix ECR cross-region auth
  [#417](https://github.com/fluxcd/image-reflector-controller/pull/417)

## 0.29.0

**Release date:** 2023-07-04

This prerelease comes with support for Kubernetes v1.27.3 and updates to the
controller's dependencies.

Starting with this version, the build, release and provenance portions of the
Flux project supply chain [provisionally meet SLSA Build Level 3](https://fluxcd.io/flux/security/slsa-assessment/).

Improvements:

- Update dependencies
  [#405](https://github.com/fluxcd/image-reflector-controller/pull/405)
- [#410](https://github.com/fluxcd/image-reflector-controller/pull/410)
- Add tests for default `v` prefix with semver policy
  [#385](https://github.com/fluxcd/image-reflector-controller/pull/385)

## 0.28.0

**Release date:** 2023-05-26

This prerelease comes with support for Kubernetes v1.27 and updates to the
controller's dependencies.

Improvements:

- Update dependencies and Kubernetes to 1.27.2
  [#378](https://github.com/fluxcd/image-reflector-controller/pull/378)
- Remove the tini supervisor
  [#379](https://github.com/fluxcd/image-reflector-controller/pull/379)
- Update workflows and enable dependabot
  [#380](https://github.com/fluxcd/image-reflector-controller/pull/380)
- Bump github/codeql-action from 2.3.3 to 2.3.4
  [#381](https://github.com/fluxcd/image-reflector-controller/pull/381)

## 0.27.2

**Release date:** 2023-05-12

This prerelease comes with updates to the controller dependencies
to patch CVE-2023-2253.

In addition, the controller base image has been updated to Alpine 3.18.

Improvements:
- Update Alpine to 3.18
  [#374](https://github.com/fluxcd/image-reflector-controller/pull/374)
- Bump github.com/docker/distribution from 2.8.1+incompatible to 2.8.2+incompatible
  [#376](https://github.com/fluxcd/image-reflector-controller/pull/376)

## 0.27.1

**Release date:** 2023-05-09

This prerelease comes with updates to the OCI related packages.

Improvements:
* Update dependencies
  [#372](https://github.com/fluxcd/image-reflector-controller/pull/372)

## 0.27.0

**Release date:** 2023-03-31

This prerelease adds support for Azure Workload Identity when using
`provider: azure` in `ImageRepository` objects.

In addition, the controller now supports horizontal scaling
using sharding based on a label selector.

The new `--watch-label-selector` lets operators provide a label to the controller manager
which in turn uses it to reconcile only those resources
(`ImageRepositories` and `ImagePolicies`) that match the given label expression.

This way operators can deploy multiple instances of IRC,
each reconciling a distinct set of resources based on their labels
and effectively scale the controller horizontally.

If sharding is enabled, all `ImagePolicy` resources can only refer
to those `ImageRepository` resources that are captured by the exact
same label selector as the `ImagePolicies`.

Improvements:
- Add reconciler sharding capability based on label selector
  [#365](https://github.com/fluxcd/image-reflector-controller/pull/365)
- Enable Workload Identity for Azure
  [#363](https://github.com/fluxcd/image-reflector-controller/pull/363)
- Move `controllers` to `internal/controllers`
  [#362](https://github.com/fluxcd/image-reflector-controller/pull/362)

## 0.26.1

**Release date:** 2023-03-20

This prerelease fixes a bug in the reconcilers due to which an error log due to
some failure may contain previous successful reconciliation message.

Fixes:
- Fix error logs with stale success message
  [#357](https://github.com/fluxcd/image-reflector-controller/pull/357)

Improvements:
- chore: migrate from k8s.gcr.io to registry.k8s.io
  [#358](https://github.com/fluxcd/image-reflector-controller/pull/358)

## 0.26.0

**Release date:** 2023-03-08

This prerelease re-instantiates the `--aws-autologin-for-ecr`,
`--gcp-autologin-for-gcr` and `--azure-autologin-for-acr` flags which became
deprecated in [`v0.25.0`](#0250), after receiving feedback of it complicating
upgrading gradually. The flags will now be removed in the future, and at least
one minor version after this release. We are sorry for any inconvenience this
may have caused.

In addition, `klog` is now configured to log using the same logger as the rest
of the controller (providing a consistent log format).

Lastly, the controller is now built with Go 1.20, and the dependencies have
been updated to their latest versions.

Improvements:
- Update Go to 1.20
  [#347](https://github.com/fluxcd/image-reflector-controller/pull/347)
- Update dependencies
  [#349](https://github.com/fluxcd/image-reflector-controller/pull/349)
  [#351](https://github.com/fluxcd/image-reflector-controller/pull/351)
- Use `logger.SetLogger` to also configure `klog`
  [#350](https://github.com/fluxcd/image-reflector-controller/pull/350)
- Fallback to autologin flags if no provider is specified
  [#353](https://github.com/fluxcd/image-reflector-controller/pull/353)


## 0.25.0

**Release date:** 2023-02-16

This prerelease graduates the `ImageRepository` and `ImagePolicy` APIs to
v1beta2.

### `image.toolkit.fluxcd.io/v1beta2`

After upgrading the controller to v0.25.0, please update the `ImageRepository`
and `ImagePolicy` **Custom Resources** in Git by replacing
`image.toolkit.fluxcd.io/v1beta1` with `image.toolkit.fluxcd.io/v1beta2` in all
YAML manifests.

### Highlights

#### New API specification format

[The specifications for the `v1beta2`
API](https://github.com/fluxcd/image-reflector-controller/tree/v0.25.0/docs/spec/v1beta2)
have been written in a new format with the aim to be more valuable to a user.
Featuring separate sections with examples, and information on how to write
and work with them.

#### Enhanced Kubernetes Conditions

`ImageRepository` and `ImagePolicy` resources will now advertise more explicit
Condition types, provide `Reconciling` and `Stalled` Conditions where applicable
for [better integration with
`kstatus`](https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md#conditions),
and record the Observed Generation on the Condition.

#### Enhanced ImageRepository scanned tags status

The `ImageRepository` objects will now show the ten latest scanned tags, which
can be helpful in troubleshooting to see a sample of the tags that have been
scanned.

```yaml
status:
  ...
  lastScanResult:
    latestTags:
    - latest
    - 6.3.3
    - 6.3.2
    - 6.3.1
    - 6.3.0
    - 6.2.3
    - 6.2.2
    - 6.2.1
    - 6.2.0
    - 6.1.8
    scanTime: "2023-02-07T19:18:01Z"
    tagCount: 41
```

#### Enhanced ImagePolicy update status

The `ImagePolicy` objects will now keep a record of the previous image in the
status and include it in the update message in the events and notifications.

Status:
```yaml
status:
  ...
  latestImage: ghcr.io/stefanprodan/podinfo:6.2.1
  observedPreviousImage: ghcr.io/stefanprodan/podinfo:6.2.0
```

Event/notification message:

```
Latest image tag for 'ghcr.io/stefanprodan/podinfo' updated from 6.2.0 to 6.2.1
```

#### :warning: Breaking changes

The autologin flags (`--aws-autologin-for-ecr`, `--gcp-autologin-for-gcr` and
`--azure-autologin-for-acr`) have been deprecated to bring the Image API closer
to the Source API, where cloud provider contextual login is configured at object
level with `.spec.provider`. Usage of these flags will result in a logged error.
Please update all the `ImageRepository` manifests that require contextual login
with the new field `.spec.provider` and the appropriate cloud provider value;
`aws`, `gcp`, or `azure`. Refer the
[docs](https://fluxcd.io/flux/components/image/imagerepositories/#provider) for
more details and examples.

### Full changelog

Improvements:
* Refactor reconcilers and introduce v1beta2 API
  [#311](https://github.com/fluxcd/image-reflector-controller/pull/311)
* Update dependencies
  [#341](https://github.com/fluxcd/image-reflector-controller/pull/341)

## 0.24.0

**Release date:** 2023-02-01

This prerelease disables caching of Secrets and ConfigMaps to improve memory
usage. To opt-out from this behavior, start the controller with:
`--feature-gates=CacheSecretsAndConfigMaps=true`.

In addition, the controller dependencies have been updated to
Kubernetes v1.26.1 and controller-runtime v0.14.2. The controller base image has
been updated to Alpine 3.17.

Improvements:
* ImagePolicy: Add predicates to filter events
  [#334](https://github.com/fluxcd/image-reflector-controller/pull/334)
* Update dependencies
  [#335](https://github.com/fluxcd/image-reflector-controller/pull/335)
* build: Enable SBOM and SLSA Provenance
  [#336](https://github.com/fluxcd/image-reflector-controller/pull/336)
* Disable caching of Secrets and ConfigMaps
  [#337](https://github.com/fluxcd/image-reflector-controller/pull/337)

## 0.23.1

**Release date:** 2022-12-20

This prerelease comes with dependency updates and improvements to the fuzzing.

Improvements:
* Update dependencies
  [#331](https://github.com/fluxcd/image-reflector-controller/pull/331)
* fuzz: Use build script from upstream
  [#330](https://github.com/fluxcd/image-reflector-controller/pull/330)
* fuzz: Improve fuzz tests' reliability
  [#329](https://github.com/fluxcd/image-reflector-controller/pull/329)

## 0.23.0

**Release date:** 2022-11-18

This prerelease comes with the removal of the `v1alpha1` and `v1alpha2` API versions which were deprecated in 2021.

Improvements:
* Use Flux Event API v1beta1
  [#321](https://github.com/fluxcd/image-reflector-controller/pull/321)
* Remove deprecated alpha APIs
  [#323](https://github.com/fluxcd/image-reflector-controller/pull/323)
* Remove nsswitch.conf creation
  [#326](https://github.com/fluxcd/image-reflector-controller/pull/326)
* Update dependencies
  [#327](https://github.com/fluxcd/image-reflector-controller/pull/327)

## 0.22.1

**Release date:** 2022-10-28

This prerelease comes with dependency updates to patch upstream CVEs.

The controller dependencies have been updated to Kubernetes v1.25.3.
The `golang.org/x/text` package has been updated to v0.4.0 (fix for CVE-2022-32149).

Improvements:
* Update dependencies
  [#319](https://github.com/fluxcd/image-reflector-controller/pull/319)

## 0.22.0

**Release date:** 2022-09-27

This prerelease comes with strict validation rules for API fields which define a
(time) duration. Effectively, this means values without a time unit (e.g. `ms`,
`s`, `m`, `h`) will now be rejected by the API server. To stimulate sane
configurations, the units `ns`, `us` and `µs` can no longer be configured, nor
can `h` be set for fields defining a timeout value.

In addition, the controller dependencies have been updated
to Kubernetes controller-runtime v0.13.

:warning: **Breaking changes:**
- `ImageRepository.spec.interval` new validation pattern is `"^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"`
- `ImageRepository.spec.timeout` new validation pattern is `"^([0-9]+(\\.[0-9]+)?(ms|s|m))+$"`

Improvements:
* api: add custom validation for v1.Duration types
  [#314](https://github.com/fluxcd/image-reflector-controller/pull/314)
* Update dependencies
  [#315](https://github.com/fluxcd/image-reflector-controller/pull/315)
* Dockerfile: Build with Go 1.19
  [#317](https://github.com/fluxcd/image-reflector-controller/pull/317)

## 0.21.0

**Release date:** 2022-09-09

This prerelease comes with improvements to fuzzing.
In addition, the controller dependencies have been updated
to Kubernetes controller-runtime v0.12.

:warning: **Breaking change:** The controller logs have been aligned
with the Kubernetes structured logging. For more details on the new logging
structure please see: [fluxcd/flux2#3051](https://github.com/fluxcd/flux2/issues/3051).

Improvements:
* Align controller logs to Kubernetes structured logging
  [#306](https://github.com/fluxcd/image-reflector-controller/pull/306)
* Refactor Fuzzers based on Go native fuzzing
  [#308](https://github.com/fluxcd/image-reflector-controller/pull/308)
* Fuzz optimisations
  [#307](https://github.com/fluxcd/image-reflector-controller/pull/307)

## 0.20.1

**Release date:** 2022-08-29

This prerelease comes with panic recovery, to protect the controller
from crashing when reconciliations lead to a crash.

In addition, the controller dependencies have been updated to Kubernetes v1.25.0.

Improvements:
* Enables RecoverPanic option on reconcilers
  [#302](https://github.com/fluxcd/image-reflector-controller/pull/302)
* Update Kubernetes packages to v1.25.0
  [#403](https://github.com/fluxcd/image-reflector-controller/pull/303)

## 0.20.0

**Release date:** 2022-08-08

This prerelease replaces the cloud provider registry auto-login code with the
new [github.com/fluxcd/pkg/oci](https://pkg.go.dev/github.com/fluxcd/pkg/oci)
package. It also comes with some minor improvements and updates dependencies to
their latest versions.

Improvements:
- tests: Move common provider helpers to tftestenv
  [#288](https://github.com/fluxcd/image-reflector-controller/pull/288)
- tests/integration: Use terraform modules and test-infra/tftestenv
  [#292](https://github.com/fluxcd/image-reflector-controller/pull/292)
- Use fluxcd/pkg/oci
  [#293](https://github.com/fluxcd/image-reflector-controller/pull/293)
- Update pkg/oci to v0.2.0
  [#295](https://github.com/fluxcd/image-reflector-controller/pull/295)
- Add flags to configure exponential back-off retry
  [#297](https://github.com/fluxcd/image-reflector-controller/pull/297)
- Update dependencies
  [#298](https://github.com/fluxcd/image-reflector-controller/pull/298)
- Skip error policy reconciliation if no tags are found
  [#300](https://github.com/fluxcd/image-reflector-controller/pull/300)

## 0.19.4

**Release date:** 2022-07-26

This prerelease comes with fix for a bug introduced in the last release during
the refactoring of the cloud provider registry auto-login. When a cloud provider
registry is identified, but is not configured for auto-login, to continue
attempting scan as public repository, an unconfigured provider error is ignored.

Fixes:
- imagerepo: Continue scan for unconfigured provider
  [#290](https://github.com/fluxcd/image-reflector-controller/pull/290)

Improvements:
- Fix the indentation issues in example
  [#286](https://github.com/fluxcd/image-reflector-controller/pull/286)
- cloud-provider-e2e: Use test image-reflector build
  [#287](https://github.com/fluxcd/image-reflector-controller/pull/287)

## 0.19.3

**Release date:** 2022-07-13

This prerelease comes with some minor improvements and updates dependencies
to patch upstream CVEs.

Fixes:

- Fix spelling mistake in azure/exchanger.go
  [#265](https://github.com/fluxcd/image-reflector-controller/pull/265)

Improvements:

- build: Upgrade to Go 1.18
  [#281](https://github.com/fluxcd/image-reflector-controller/pull/281)
- Add native registry login tests for EKS, AKS and GKE
  [#275](https://github.com/fluxcd/image-reflector-controller/pull/275)
- Introduce registry package
  [#276](https://github.com/fluxcd/image-reflector-controller/pull/276)
- tests/int: ECR force delete and use go 1.18
  [#282](https://github.com/fluxcd/image-reflector-controller/pull/282)
- Update dependencies
  [#280](https://github.com/fluxcd/image-reflector-controller/pull/280)
  [#283](https://github.com/fluxcd/image-reflector-controller/pull/283)


## 0.19.2

**Release date:** 2022-06-24

This prerelease comes with finalizers to properly record the reconciliation metrics for deleted resources.

Improvements:
- Add finalizers to `ImagePolicy` and `ImageRepository` resources
  [#266](https://github.com/fluxcd/image-reflector-controller/pull/266)

Fixes:
- Fix response body read and close defer order
  [#272](https://github.com/fluxcd/image-reflector-controller/pull/272)
- Use unique resources in tests
  [#279](https://github.com/fluxcd/image-reflector-controller/pull/279)

## 0.19.1

**Release date:** 2022-06-08

This prerelease comes with improvements to the `ImageRepository` validation.

In addition, the controller dependencies where update to Kubernetes v1.24.1.

Improvements:
- Validate that the image name does not contain tags
  [#268](https://github.com/fluxcd/image-reflector-controller/pull/268)
- Update dependencies
  [#269](https://github.com/fluxcd/image-reflector-controller/pull/269)

## 0.19.0

**Release date:** 2022-05-27

This prerelease adds support for excluding certain tags when defining `ImageRepositories`.
The `spec.exclusionList` field can be used to specify a list of regex expressions.
If the exclusion list is empty, by default the regex `"^.*\\.sig$"` is used
to exclude all tags ending with `.sig`, since these are
[cosign](https://github.com/sigstore/cosign) OCI artifacts and not container
images which can be deployed on a Kubernetes cluster.

Features:
- Add `exclusionList` to ImageRepository API
  [#256](https://github.com/fluxcd/image-reflector-controller/pull/256)

Improvements:
- Update dependencies
  [#258](https://github.com/fluxcd/image-reflector-controller/pull/258)
  [#261](https://github.com/fluxcd/image-reflector-controller/pull/261)
- Update Alpine to 3.16
  [#262](https://github.com/fluxcd/image-reflector-controller/pull/262)

## 0.18.0

**Release date:** 2022-05-03

This prerelease adds support for defining a `.spec.serviceAccountName` in
`ImageRepository` objects. When specified, the image pull secrets attached to
the ServiceAccount are used to authenticate towards the registry.

Features:
- Add `serviceAccountName` to ImageRepository API
  [#252](https://github.com/fluxcd/image-reflector-controller/pull/252)
  [#253](https://github.com/fluxcd/image-reflector-controller/pull/253)

Improvements:
- Update dependencies
  [#254](https://github.com/fluxcd/image-reflector-controller/pull/254)

Other notable changes:
- Rewrite all the tests to testenv with gomega
  [#249](https://github.com/fluxcd/image-reflector-controller/pull/249)

## 0.17.2

**Release date:** 2022-04-19

This prerelease updates dependencies to their latest versions.

Improvements:
- Update dependencies
  [#247](https://github.com/fluxcd/image-reflector-controller/pull/247)

Fixes:
- Align version of dependencies when Fuzzing
  [#243](https://github.com/fluxcd/image-reflector-controller/pull/243)

## 0.17.1

**Release date:** 2022-03-23

This prerelease ensures the API objects fully adhere to newly introduced
interfaces, allowing them to work in combination with e.g. the
[`conditions`](https://pkg.go.dev/github.com/fluxcd/pkg/runtime@v0.13.2/conditions)
package.

Improvements:
- Implement `meta.ObjectWithConditions` interfaces
  [#241](https://github.com/fluxcd/image-reflector-controller/pull/241)

## 0.17.0

**Release date:** 2022-03-21

This prerelease updates various dependencies to their latest versions, thereby
eliminating at least 13 OSVs, and preparing the code base for more standardized
controller runtime operations.

In addition, the Azure Scope has been fixed to work correctly with Azure
Environment Credentials.

Improvements:
- Refactor logging to be more consistent
  [#232](https://github.com/fluxcd/image-reflector-controller/pull/232)
- Update dependencies
  [#234](https://github.com/fluxcd/image-reflector-controller/pull/234)
  [#236](https://github.com/fluxcd/image-reflector-controller/pull/236)
  [#238](https://github.com/fluxcd/image-reflector-controller/pull/238)
- Update `pkg/runtime` and `apis/meta`
  [#235](https://github.com/fluxcd/image-reflector-controller/pull/235)

Fixes:
- Invalid Azure Scope
  [#231](https://github.com/fluxcd/image-reflector-controller/pull/231)
- Refactor registry test code and fix fuzz integration
  [#233](https://github.com/fluxcd/image-reflector-controller/pull/233)
- Run tidy before Go test
  [#240](https://github.com/fluxcd/image-reflector-controller/pull/240)

## 0.16.0

**Release date:** 2022-01-31

This prerelease comes with support for automatically getting
credentials from Azure and Google Cloud when scanning images in ACR and GCR.
To configure autologin for ACR, ECR or GCR please see the
[cloud providers authentication guide](https://fluxcd.io/flux/guides/image-update/#imagerepository-cloud-providers-authentication).

Platform admins can disable cross-namespace references with the
`--no-cross-namespace-refs=true` flag. When this flag is set,
image policies can only refer to image repositories in the same namespace
as the policy object, preventing tenants from accessing another tenant's repositories.

Starting with this version, the controller deployment conforms to the
Kubernetes [restricted pod security standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted):
- all Linux capabilities were dropped
- the root filesystem was set to read-only
- the seccomp profile was set to the runtime default
- run as non-root was enabled
- the user and group ID was set to 65534

**Breaking changes**:
- The use of new seccomp API requires Kubernetes 1.19.
- The controller container is now executed under 65534:65534 (userid:groupid).
  This change may break deployments that hard-coded the user ID of 'controller' in their PodSecurityPolicy.

Features:
- Get credentials from GCP/Azure when needed
  [#194](https://github.com/fluxcd/image-reflector-controller/pull/194)
- Allow disabling cross-namespace references to image repositories
  [#228](https://github.com/fluxcd/image-reflector-controller/pull/228)

Improvements:
- Publish SBOM and sign release artifacts
  [#227](https://github.com/fluxcd/image-reflector-controller/pull/227)
- Drop capabilities, enable seccomp and enforce runAsNonRoot
  [#223](https://github.com/fluxcd/image-reflector-controller/pull/223)
- Refactor Fuzz implementation
  [#221](https://github.com/fluxcd/image-reflector-controller/pull/221)
- Clarifications for auto-login feature
  [#219](https://github.com/fluxcd/image-reflector-controller/pull/219)

Fixes:
- Fix scheme validation check when using host:port
  [#222](https://github.com/fluxcd/image-reflector-controller/pull/222)
- Fix makefile envtest and controller-gen usage
  [#218](https://github.com/fluxcd/image-reflector-controller/pull/218)

## 0.15.0

**Release date:** 2022-01-07

This prerelease comes with an update to the Kubernetes and controller-runtime dependencies
to align them with the Kubernetes 1.23 release.

In addition, the controller is now built with Go 1.17 and Alpine 3.15.

Improvements:
- Update Go to v1.17
  [#190](https://github.com/fluxcd/image-reflector-controller/pull/190)
- Add various instructions on development documentation
  [#215](https://github.com/fluxcd/image-reflector-controller/pull/215)

## 0.14.0

**Release date:** 2021-11-23

This prerelease updates Alpine to v3.14, and several dependencies to their latest
version. Solving an issue with `rest_client_request_latency_seconds_.*` high
cardinality metrics.

To enhance the experience of consumers observing the `ImagePolicy` and `ImageRepository`
objects using `kstatus`, a default of `-1` is now configured for the `observedGeneration`
to ensure it does not report a false positive in the time the controller has not marked
the resource with a `Ready` condition yet.

Improvements:
- Set default observedGeneration to -1
  [#189](https://github.com/fluxcd/image-reflector-controller/pull/189)
- Update Alpine to v3.14
  [#203](https://github.com/fluxcd/image-reflector-controller/pull/203)
- Update dependencies
  [#204](https://github.com/fluxcd/image-reflector-controller/pull/204)
- Update github.com/opencontainers/image-spec to v1.0.2
  [#205](https://github.com/fluxcd/image-reflector-controller/pull/205)

## 0.13.2

**Release date**: 2021-11-12

This prerelease comes with a regression bug fix for when policies reference repositories in the same namespace.

Fixes:
* Fix watched same-ns image repos trigger reconcile
  [#199](https://github.com/fluxcd/image-reflector-controller/pull/199)

## 0.13.1

**Release date**: 2021-11-11

This prerelease comes with a bug fix for when policies reference repositories across namespaces.

Fixes:
* Watched cross-ns image repos trigger reconcile
  [#196](https://github.com/fluxcd/image-reflector-controller/pull/196)

## 0.13.0

**Release date**: 2021-10-19

This prerelease adds experimental support for automatically getting
credentials from AWS when scanning an image in [Elastic Container
Registry
(ECR)](https://docs.aws.amazon.com/AmazonECR/latest/userguide/what-is-ecr.html).

Improvements:
* Get credentials from AWS ECR when needed
  [#174](https://github.com/fluxcd/image-reflector-controller/pull/174)

## 0.12.0

**Release date:** 2021-10-08

This prerelease comes with an (experimental) introduction of ACLs for allowing cross-namespace
access to `ImageRepository` resources. You can read more about how they work in the
[pull request](https://github.com/fluxcd/image-reflector-controller/pull/162) that
introduced them.

In addition, a bug has been fixed that caused the controller to segfault when a malformed
SemVer was defined.

Improvements:
* [RFC] Add ACL support for allowing cross-namespace access to image repository
  [#162](https://github.com/fluxcd/image-reflector-controller/pull/162)

Fixes:
* policy: Handle failure due to invalid semver range
  [#172](https://github.com/fluxcd/image-reflector-controller/pull/172)

## 0.11.1

**Release date:** 2021-08-05

This prerelease comes with an update to the Kubernetes and controller-runtime
dependencies to align them with the Kubernetes `v1.21.3` release, including an update
of Badger to `v3.2103.1`.

Improvements:
* Update dependencies
  [#160](https://github.com/fluxcd/image-reflector-controller/pull/160)

## 0.11.0

**Release date:** 2021-06-28

This prerelease promotes the API version from `v1alpha2` to `v1beta1`.

:warning: With regard to the API version, no action is necessary at
present, as Kubernetes will automatically convert between `v1alpha2`
and `v1beta1` APIs.

You may wish to migrate `v1alpha2` YAML files to `v1beta1`, in
preparation for `v1alpha2` being deprecated (eventually; there is no
date set at the time of writing). This is simply a case of setting the
`apiVersion` field value:

    `apiVersion: image.toolkit.fluxcd.io/v1beta1`

Improvements:
* Let people set the number of controller workers with a flag
  [#153](https://github.com/fluxcd/image-reflector-controller/pull/153)

## 0.10.0

**Release date:** 2021-06-10

This prerelease comes with an update to the Kubernetes and controller-runtime
dependencies to align them with the Kubernetes 1.21 release, including an update
of Badger to `v3.2103.0`.

Improvements:
* Better error reporting for image policy evaluation
  [#144](https://github.com/fluxcd/image-reflector-controller/pull/144)
* Update Go and Badger
  [#149](https://github.com/fluxcd/image-reflector-controller/pull/149)
* Update dependencies
  [#150](https://github.com/fluxcd/image-reflector-controller/pull/150)
* Add nightly builds workflow and allow RC releases
  [#151](https://github.com/fluxcd/image-reflector-controller/pull/151)

## 0.9.1

**Release date:** 2021-04-29

This prerelease comes with improvements to error reporting.

Fixes:
* Ensure invalid regex errors are reported to user
  [#140](https://github.com/fluxcd/image-reflector-controller/pull/140)
* Remove v1alpha1 API from Scheme
  [#136](https://github.com/fluxcd/image-reflector-controller/pull/136)

## 0.9.0

**Release date:** 2021-04-21

This prerelease comes with breaking changes to the `image.toolkit.fluxcd.io` APIs.

The `v1alpha1` APIs have been promoted to `v1alpha2`, while the version has
changed the API definitions have not, and upgrading can be done by changing
the version in your manifests for the `ImageRepository` and `ImagePolicy` kinds.

Improvements:
* Move API v1alpha1 to v1alpha2
  [#132](https://github.com/fluxcd/image-reflector-controller/pull/132)
* Add API docs for v1alpha2
  [#134](https://github.com/fluxcd/image-reflector-controller/pull/134)

Fixes:
* Parse docker auths and use only hostname
  [#119](https://github.com/fluxcd/image-reflector-controller/pull/119)
  
## 0.8.0

**Release date:** 2021-04-06

This prerelease comes with a breaking change to the leader election ID
from `e189b2df.fluxcd.io` to `image-reflector-controller-leader-election`
to be more descriptive. This change should not have an impact on most
installations, as the default replica count is `1`. If you are running
a setup with multiple replicas, it is however advised to scale down
before upgrading.

The controller exposes a gauge metric to track the suspended status
of `ImageRepository` objects: `gotk_suspend_status{kind,name,namespace}`.

Improvements:
* Set leader election deadline to 30s
  [#125](https://github.com/fluxcd/image-reflector-controller/pull/125)
* Record suspension metrics
  [#123](hhttps://github.com/fluxcd/image-reflector-controller/pull/123)

## 0.7.1

**Release date:** 2021-03-16

This prerelease comes with updates to the runtime packages.

Improvements:
* Update dependencies
  [#121](https://github.com/fluxcd/image-reflector-controller/pull/121)

Fixes:
* Fix `last scan` print column for `ImageRepository`
  [#119](https://github.com/fluxcd/image-reflector-controller/pull/119)

## 0.7.0

**Release date:** 2021-02-24

This prerelease comes with various updates to the controller's
dependencies; most notable the `go-containerregistry` library
was upgrade from `v0.1.1` to `v0.4.0`.

The Kubernetes custom resource definitions are packaged as
a multi-doc YAML asset and published on the GitHub release page.

Improvements:
* Refactor release workflow
  [#110](https://github.com/fluxcd/image-reflector-controller/pull/110)
* Update dependencies
  [#109](https://github.com/fluxcd/image-reflector-controller/pull/109)

## 0.6.0

**Release date:** 2021-02-12

This prerelease comes with support for defining policies
with numerical ordering.

Features:
* Implement numerical ordering policy
  [#104](https://github.com/fluxcd/image-reflector-controller/pull/104)
  [#106](https://github.com/fluxcd/image-reflector-controller/pull/106)

Improvements:
* Enable pprof endpoints on metrics server
  [#100](https://github.com/fluxcd/image-reflector-controller/pull/100)
* Update Alpine to v3.13
  [#101](https://github.com/fluxcd/image-reflector-controller/pull/101)

## 0.5.0

**Release date:** 2021-02-01

This prerelease comes with support for supplying a client cert, key
and CA (self-singed TLS) to be used for authentication with
container image registries.

## 0.4.1

**Release date:** 2021-01-22

This prerelease comes with a new argument flag to set the database's
memory mapped value log file size in bytes (`--storage-value-log-file-size`),
with a 32bit ARMv7 friendly default of `1<<28` (`256MiB`).

## 0.4.0

**Release date:** 2021-01-21

This prerelease comes with two new argument flags,
introduced to support configuring the QPS
(`--kube-api-qps`) and burst (`--kube-api-burst`) while communicating
with the Kubernetes API server.

The `LocalObjectReference` from the Kubernetes core has been replaced
with our own, making the `name` a required field. The impact of this
should be limited to direct API consumers only, as the field was
already required by controller logic.

## 0.3.0

**Release date:** 2021-01-16

This prerelease comes with updates to Kubernetes and Badger dependencies.
The Kubernetes packages were updated to v1.20.2 and Badger to v3.2011.0.

## 0.2.0

**Release date:** 2021-01-13

This is the second MINOR prerelease, adding support for [selecting
images using regular expressions][regex].

Other notable changes:

- `controller-runtime` dependency has been upgraded to `v0.7.0`.
- The container image for ARMv7 and ARM64 that used to be published
  separately as `image-reflector-controller:*-arm64` has been merged
  with the AMD64 image.

[regex]: https://github.com/fluxcd/image-reflector-controller/pull/75

## 0.1.0

**Release date:** 2020-12-10

This is the first prerelease of image-reflector-controller and its
API. The purpose of the controller is to scan image repositories, and
calculate a "latest image" according to some specification. Automation
(e.g., the [image-automation-controller][auto-controller]) can use
that information to run updates, so that the latest image is deployed.

The controller and API conform to the conventions of the GitOps
Toolkit, so will be compatible with (and soon, included in) the `flux`
CLI and dashboards and so on.

This release supports:

 - supplying a docker-registry secret as credentials for accessing an
   image repository
 - selecting images according to a [semver][semver] range.
 - selecting images according to alphabetical order (ascending or
   descending)
 - keeping the database on a volume (e.g., a PersistentVolumeClaim) so
   that it survives restarts

[semver]: https://github.com/Masterminds/semver#basic-comparisons
[auto-controller]: https://github.com/fluxcd/image-automation-controller
