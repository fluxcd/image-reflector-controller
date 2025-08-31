# Development

> **Note:** Please take a look at <https://fluxcd.io/contributing/flux/>
> to find out about how to contribute to Flux and how to interact with the
> Flux Development team.

## Installing required dependencies
There are a number of dependencies required to be able to run image-reflector-controller and its test-suite locally. 
* [Install Go](https://golang.org/doc/install)
* [Install Kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
* [Install Docker](https://docs.docker.com/engine/install/)
* (Optional) [Install Kubebuilder](https://book.kubebuilder.io/quick-start.html)

## How to run the test suite

Prerequisites:
* go >= 1.25
* kustomize >= 3.1

You can run them by simply doing

```bash
make test
```

> **Note:** Since this will also trigger generating some files such as manifests, it is advised to run this prior to committing your changes, especially when making API changes.

> Please refer to the Makefile to see all make targets and what they do.

## How to install the controller

You can install the CRDs and the controller by simply doing

```bash
# Install CRDs into a cluster
make install
# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
make deploy
```

## How to run the controller locally

You can run the controller on your host by

```bash
make run
```

## How to generate and update CRDs API reference documentation

If you made any changes to CRDs API, you can update CRDs API reference doc by

```bash
make api-docs
```