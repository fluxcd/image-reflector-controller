# Changelog

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
