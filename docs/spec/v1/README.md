# image.toolkit.fluxcd.io/v1

This is the v1 API specification for defining image scanning and update policies.

## Specification

* Image kinds:
  + [ImageRepository](imagerepositories.md)
  + [ImagePolicy](imagepolicies.md)

## Implementation

* [image-reflector-controller](https://github.com/fluxcd/image-reflector-controller)

## Consumers

* [image-automation-controller](https://github.com/fluxcd/image-automation-controller)
