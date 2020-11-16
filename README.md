# Image (metadata) reflector controller

This is a controller that reflects container image metadata into a
Kubernetes cluster. It pairs with the [image update automation][auto]
controller to drive automated config updates.

## Installing

Instructions for setting both controllers up are in the
[fluxcd/image-automation-controller][auto] README.

If you just want to run this controller, do

```bash
kustomize build github.com/fluxcd/image-reflector-controller//config/default/?ref=main | kubectl apply -f-
```

[auto]: https://github.com/fluxcd/image-automation-controller
