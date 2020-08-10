# Image (metadata) reflector controller

This is an attempt to build controllers along the lines set out in
https://squaremo.dev/posts/gitops-controllers/.

This repository implements the image metadata reflector controller,
which scans container image repositories and reflects the metadata, in
Kubernetes resources. The sibling repository
[image-automation-controller](https://github.com/fluxcd/image-automation-controller)
implements the automation controller, which acts on the reflected data
(e.g., a new image version) by updating the image references used in
files in git.
