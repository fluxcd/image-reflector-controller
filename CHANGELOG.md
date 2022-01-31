# Changelog

## 0.16.0

**Release date:** 2022-01-31

This prerelease comes with support for automatically getting
credentials from Azure and Google Cloud when scanning images in ACR and GCR.
To configure autologin for ACR, ECR or GCR please see the
[cloud providers authentication guide](https://fluxcd.io/docs/guides/image-update/#imagerepository-cloud-providers-authentication).

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
