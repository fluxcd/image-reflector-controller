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

## Debugging the controller locally

When reproducing an issue or stepping through reconciliation logic, the
following knobs make local runs cheaper and the resulting logs easier to
read.

### Limit the watched namespace

The controller watches every namespace by default. To narrow it to a single
namespace, set the `RUNTIME_NAMESPACE` environment variable before invoking
`make run`:

```sh
RUNTIME_NAMESPACE=flux-system make run
```

### Reduce reconcile concurrency

Each `ImageRepository` and `ImagePolicy` reconcile is processed concurrently
(default `--concurrent=4`). When debugging it is almost always easier to
follow a serial trace; pass `--concurrent=1` so reconciles run one at a
time:

```sh
go run ./main.go --concurrent=1
```

### Suspend unrelated objects

If the controller is sharing a cluster with other Flux objects, suspend
anything not relevant to the test you're running so their reconciles don't
interleave with yours:

```sh
flux suspend image repository <name>
flux suspend image policy <name>
```

Resume with `flux resume image repository|policy <name>` when you're done.
