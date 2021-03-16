# Changelog

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
