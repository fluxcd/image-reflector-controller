# Image (metadata) reflector controller

[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/4790/badge)](https://bestpractices.coreinfrastructure.org/projects/4790)
[![report](https://goreportcard.com/badge/github.com/fluxcd/image-reflector-controller)](https://goreportcard.com/report/github.com/fluxcd/image-reflector-controller)
[![license](https://img.shields.io/github/license/fluxcd/image-reflector-controller.svg)](https://github.com/fluxcd/image-reflector-controller/blob/main/LICENSE)
[![release](https://img.shields.io/github/release/fluxcd/image-reflector-controller/all.svg)](https://github.com/fluxcd/image-reflector-controller/releases)

This is a controller that reflects container image metadata into a
Kubernetes cluster. It pairs with the [image update automation][auto]
controller to drive automated config updates.

## Installing

Please see the [installation and use
guide](https://toolkit.fluxcd.io/guides/image-update/).

If you just want to run this controller for development purposes, do

```bash
kubectl kustomize https://github.com/fluxcd/image-reflector-controller.git//config/default/?ref=main | kubectl apply -f-
```

[auto]: https://github.com/fluxcd/image-automation-controller
