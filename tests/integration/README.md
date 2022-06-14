# Cloud Provider Integration Tests

## Requirements

### AWS

- AWS account with access key ID and secret access key with permissions to
    create EKS cluster and ECR repository.
- AWS CLI, need not be configured with the AWS account.
- Docker CLI for registry login.
- kubectl for applying certain install manifests.

## Test setup

Copy `.env.sample` to `.env`, put the respective provider configurations in the
environment variables and source it, `source .env`.

Run the test with `make test-*`:

```console
$ make test-aws
mkdir -p build/flux
curl -Lo build/flux/install.yaml https://github.com/fluxcd/flux2/releases/latest/download/install.yaml
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  351k  100  351k    0     0   247k      0  0:00:01  0:00:01 --:--:-- 3609k
cp kustomization.yaml build/flux
cd build/flux && kustomize edit set image fluxcd/image-reflector-controller=fluxcd/image-reflector-controller
kustomize build build/flux > build/flux.yaml
go test -timeout 20m -v ./... -existing
2022/06/15 01:55:09 Terraform binary:  /go/src/github.com/fluxcd/image-reflector-controller/tests/integration/build/terraform
2022/06/15 01:55:09 Init Terraform
2022/06/15 01:55:14 Applying Terraform
2022/06/15 01:55:41 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.0
2022/06/15 01:55:45 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.2
2022/06/15 01:55:48 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.3
2022/06/15 01:55:51 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.4
2022/06/15 01:55:54 Installing flux
=== RUN   TestImageRepositoryScan
=== RUN   TestImageRepositoryScan/ecr
--- PASS: TestImageRepositoryScan (2.15s)
    --- PASS: TestImageRepositoryScan/ecr (2.15s)
PASS
2022/06/15 01:56:14 Destroying environment...
ok      github.com/fluxcd/image-reflector-controller/tests/integration  1673.225s
```

In the above, the test created a build directory `build/` and downloaded the
latest flux install manifest at `build/flux/install.yaml`. This will be used to
install flux in the test cluster. The manifest download can be configured by
setting the `FLUX_MANIFEST_URL` variable. Once downloaded, the file can be
manually modified, if needed, it won't be downloaded again unless it's deleted.

Then the `kustomization.yaml` is copied to `build/flux/`. This kustomization
contains configurations to configure the flux installation by patching the
downloaded `install.yaml`. It can also be used to set any custom images for any
of the flux components. The image-reflector-controller image can be configured
by setting the `IMG` variable when running the test. The kustomization is built
and the resulting flux installation manifest is written to `build/flux.yaml`.
This is used by the test to install flux.

The go test is started with a long timeout because the infrastructure set up
can take a long time. It can also be configured by setting the variable
`TEST_TIMEOUT`. The test creates a new infrastructure using `tftestenv`, like
`testenv` but helps create kubernetes cluster using terraform. It looks for any
existing terraform binary on the current `$PATH` and downloads a new binary in
`build/terraform` if it couldn't find one locally.
The terraform configurations are present in `terraform/<provider>` directory.
All the terraform state created by the test run are written in
`terraform/<provider>` directory. The test creates a managed kubernetes cluster
and a container registry (with optional repository in some cases). The
repository is populated with a few randomly generated test images. The registry
login is performed using the cloud provider CLI and docker CLI. The credentials
are written into the default docker client config file. Flux is then installed
using the initial `build/flux.yaml` manifest.

Once the environment is ready, the individual go tests are executed. After the
tests end, the environment is destroyed automatically.

**IMPORTANT**: In case the terraform infrastructure results in a bad state,
maybe due to a crash during the apply, the whole infrastructure can be destroyed
by running `terraform destroy` in `terraform/<provider>` directory.

## Debugging the tests

For debugging environment provisioning, enable verbose output with `-verbose`
test flag.

```console
$ make test-aws GO_TEST_ARGS="-verbose"
```

The test environment is destroyed at the end by default. Run the tests with
`-retain` flag to retain the created test infrastructure.

```console
$ make test-aws GO_TEST_ARGS="-retain"
```

The tests require the infrastructure state to be clean. For re-running the tests
with a retained infrastructure, set `-existing` flag.

```console
$ make test-aws GO_TEST_ARGS="-retain -existing"
```

To delete an existing infrastructure created with `-retain` flag:

```console
$ make test-aws GO_TEST_ARGS="-existing"
```
