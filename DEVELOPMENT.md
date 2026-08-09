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

`make run` starts the binary against the cluster in your current kubeconfig
and sets `--storage-path=./data` so tag metadata is written under the local
checkout instead of the in-cluster default (`/data`). With the default flags
the process also listens for:

| Endpoint | Default address | Purpose |
| --- | --- | --- |
| Health probes | `:9440` (`--health-addr`) | Liveness / readiness |
| Metrics / pprof | `:8080` (`--metrics-addr`) | Prometheus metrics and pprof handlers |

## Debugging the controller locally

Use this section when you need to reproduce a reported issue or step through
`ImageRepository` / `ImagePolicy` reconciliation against a real cluster.

### Avoid racing an in-cluster controller

If the cluster already runs `image-reflector-controller`, scale it down before
starting your local process so only one instance reconciles objects and writes
to storage:

```sh
kubectl -n flux-system scale deploy/image-reflector-controller --replicas=0
```

Restore it when you are done:

```sh
kubectl -n flux-system scale deploy/image-reflector-controller --replicas=1
```

### Suspend objects that are not part of the reproduction

Shared clusters often have many `ImageRepository` and `ImagePolicy` objects.
Suspend everything you do not need so their reconciles (and registry traffic)
do not interleave with the case you are debugging:

```sh
flux suspend image-repository --all
flux suspend image-policy --all
```

`--all` applies to the current kubeconfig namespace (commonly `flux-system`).
Resume specific objects (or use `flux resume <kind> --all`) when finished.

### Increase log verbosity

`make run` uses the default `info` level. For a console-friendly local trace,
keep the same storage path as `make run` and raise verbosity:

```sh
go run ./main.go --storage-path=./data --log-level=debug --log-encoding=console
```

Supported `--log-level` values are `trace`, `debug`, `info`, and `error`.

### Exercise a minimal ImageRepository / ImagePolicy pair

Create (or leave unsuspended) only the objects under test, then watch the local
process logs while they reconcile. Useful checks:

```sh
kubectl get imagerepository <name> -o yaml
kubectl get imagepolicy <name> -o yaml
```

Confirm the `ImageRepository` status lists recent tags and the `ImagePolicy`
status shows the elected image (and digest, when reflection is enabled). Tag
metadata for the local process is stored under `./data` (see `--storage-path`
above).

### Cross-controller interactions

`image-reflector-controller` scans registries and elects tags; it does not
write to Git. Downstream, [`image-automation-controller`](https://github.com/fluxcd/image-automation-controller)
consumes `ImagePolicy` status and patches image refs in Git.

When debugging against a shared cluster, keep watching all namespaces (the
default `--watch-all-namespaces=true`). Narrowing the cache with
`--watch-all-namespaces=false` / `RUNTIME_NAMESPACE` hides cross-namespace
`ImagePolicy` → `ImageRepository` refs that often matter in reproductions, so
it is a poor default for local debugging. Prefer suspending unrelated objects
instead.

If you need to see how automation reacts to a new elected tag, run or observe
`image-automation-controller` separately after the local reflector has updated
the `ImagePolicy` status.

### Debugging with VS Code

Create a `.vscode/launch.json` file:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch image-reflector-controller",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/main.go",
            "args": [
                "--storage-path=./data",
                "--log-level=debug",
                "--log-encoding=console"
            ]
        }
    ]
}
```

Scale down the in-cluster Deployment first, then start debugging with
**Run → Start Debugging**.

## How to generate and update CRDs API reference documentation

If you made any changes to CRDs API, you can update CRDs API reference doc by

```bash
make api-docs
```