# Image (metadata) reflector controller

[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/4790/badge)](https://bestpractices.coreinfrastructure.org/projects/4790)

This is a controller that reflects container image metadata into a
Kubernetes cluster. It pairs with the [image update automation][auto]
controller to drive automated config updates.

## Installing

Please see the [installation and use
guide](https://toolkit.fluxcd.io/guides/image-update/).

If you just want to run this controller for development purposes, do

```bash
kustomize build github.com/fluxcd/image-reflector-controller//config/default/?ref=main | kubectl apply -f-
```

[auto]: https://github.com/fluxcd/image-automation-controller
