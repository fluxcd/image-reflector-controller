# image-reflector-controller

[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/4790/badge)](https://bestpractices.coreinfrastructure.org/projects/4790)
[![report](https://goreportcard.com/badge/github.com/fluxcd/image-reflector-controller)](https://goreportcard.com/report/github.com/fluxcd/image-reflector-controller)
[![license](https://img.shields.io/github/license/fluxcd/image-reflector-controller.svg)](https://github.com/fluxcd/image-reflector-controller/blob/main/LICENSE)
[![release](https://img.shields.io/github/release/fluxcd/image-reflector-controller/all.svg)](https://github.com/fluxcd/image-reflector-controller/releases)

The image-reflector-controller is a [GitOps toolkit](https://fluxcd.io/flux/components/) controller
that extends [Flux](https://github.com/fluxcd/flux2) with container image scanning and update policy capabilities.

![overview](https://fluxcd.io/img/image-update-automation.png)

The image-reflector-controller and [image-update-automation](https://github.com/fluxcd/image-automation-controller)
work together to update Git repositories when new container images are available.

- The image-reflector-controller scans image repositories and reflects the image metadata in Kubernetes resources.
- The image-automation-controller updates YAML files based on the latest images scanned, and commits the changes to a given Git repository.

## API Specification

| Kind                                                 | API Version                  |
|------------------------------------------------------|------------------------------|
| [ImageRepository](docs/spec/v1/imagerepositories.md) | `image.toolkit.fluxcd.io/v1` |
| [ImagePolicy](docs/spec/v1/imagepolicies.md)         | `image.toolkit.fluxcd.io/v1` |

## Guides

* [Get started with Flux](https://fluxcd.io/flux/get-started/)
* [Automate image updates to Git](https://fluxcd.io/flux/guides/image-update/)

## Roadmap

The roadmap for the Flux family of projects can be found at <https://fluxcd.io/roadmap/>.

## Contributing

This project is Apache 2.0 licensed and accepts contributions via GitHub pull requests.
To start contributing please see the [development guide](DEVELOPMENT.md).
