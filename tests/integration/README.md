# Cloud Provider Integration Tests

## Requirements

### Amazon Web Services

- AWS account with access key ID and secret access key with permissions to
    create EKS cluster and ECR repository.
- AWS CLI, does not need to be configured with the AWS account.
- Docker CLI for registry login.
- kubectl for applying certain install manifests.

### Microsoft Azure

- Azure account with an active subscription to be able to create AKS and ACR,
    and permission to assign roles. Role assignment is required for allowing AKS
    workloads to access ACR.
- Azure CLI, need to be logged in using `az login` as a User (not a Service
  Principal).

  **NOTE:** To use Service Principal (for example in CI environment), set the
  `ARM-*` variables in `.env`, source it and authenticate Azure CLI with:
  ```console
  $ az login --service-principal -u $ARM_CLIENT_ID -p $ARM_CLIENT_SECRET --tenant $ARM_TENANT_ID
  ```
  In this case, the AzureRM client in terraform uses the Service Principal to
  authenticate and the Azure CLI is used only for authenticating with ACR for
  logging in and pushing container images. Attempting to authenticate terraform
  using Azure CLI with Service Principal results in the following error:
  > Authenticating using the Azure CLI is only supported as a User (not a Service Principal).
- Docker CLI for registry login.
- kubectl for applying certain install manifests.

#### Permissions

Following permissions are needed for provisioning the infrastructure and running
the tests:
- `Microsoft.Kubernetes/*`
- `Microsoft.Resources/*`
- `Microsoft.Authorization/roleAssignments/{Read,Write,Delete}`
- `Microsoft.ContainerRegistry/*`
- `Microsoft.ContainerService/*`

#### IAM and CI setup

To create the necessary IAM role with all the permissions, set up CI secrets and
variables using
[azure-gh-actions](https://github.com/fluxcd/test-infra/tree/main/tf-modules/azure/github-actions)
use the terraform configuration below. Please make sure all the requirements of
azure-gh-actions are followed before running it.

**NOTE:** When running the following for a repo under an organization, set the
environment variable `GITHUB_ORGANIZATION` if setting the `owner` in the
`github` provider doesn't work.

```hcl
provider "github" {
  owner = "fluxcd"
}

module "azure_gh_actions" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/azure/github-actions"

  azure_owners          = ["owner-id-1", "owner-id-2"]
  azure_app_name        = "irc-e2e"
  azure_app_description = "irc e2e"
  azure_permissions = [
    "Microsoft.Kubernetes/*",
    "Microsoft.Resources/*",
    "Microsoft.Authorization/roleAssignments/Read",
    "Microsoft.Authorization/roleAssignments/Write",
    "Microsoft.Authorization/roleAssignments/Delete",
    "Microsoft.ContainerRegistry/*",
    "Microsoft.ContainerService/*"
  ]
  azure_location = "eastus"

  github_project = "image-reflector-controller"

  github_secret_client_id_name       = "IRC_E2E_AZ_ARM_CLIENT_ID"
  github_secret_client_secret_name   = "IRC_E2E_AZ_ARM_CLIENT_SECRET"
  github_secret_subscription_id_name = "IRC_E2E_AZ_ARM_SUBSCRIPTION_ID"
  github_secret_tenant_id_name       = "IRC_E2E_AZ_ARM_TENANT_ID"
}
```

**NOTE:** The environment variables used above are for the GitHub workflow that
runs the tests. Change the variable names if needed accordingly.

### Google Cloud Platform

- GCP account with project and GKE, GCR and Artifact Registry services enabled
    in the project.
- gcloud CLI, need to be logged in using `gcloud auth login` as a User (not a
  Service Account), configure application default credentials with `gcloud auth
  application-default login` and docker credential helper with `gcloud auth configure-docker`.

  **NOTE:** To use Service Account (for example in CI environment), set
  `GOOGLE_APPLICATION_CREDENTIALS` variable in `.env` with the path to the JSON
  key file, source it and authenticate gcloud CLI with:
  ```console
  $ gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS
  ```
  Depending on the Container/Artifact Registry host used in the test, authenticate
  docker accordingly
  ```console
  $ gcloud auth print-access-token | docker login -u oauth2accesstoken --password-stdin https://us-central1-docker.pkg.dev
  $ gcloud auth print-access-token | docker login -u oauth2accesstoken --password-stdin https://gcr.io
  ```
  In this case, the GCP client in terraform uses the Service Account to
  authenticate and the gcloud CLI is used only to authenticate with Google
  Container Registry and Google Artifact Registry.

  **NOTE FOR CI USAGE:** When saving the JSON key file as a CI secret, compress
  the file content with
  ```console
  $ cat key.json | jq -r tostring
  ```
  to prevent aggressive masking in the logs. Refer
  [aggressive replacement in logs](https://github.com/google-github-actions/auth/blob/v1.1.0/docs/TROUBLESHOOTING.md#aggressive--replacement-in-logs)
  for more details.
- Docker CLI for registry login.
- kubectl for applying certain install manifests.

**NOTE:** Unlike ECR, ACR and Google Artifact Registry, Google Container
Registry tests don't create a new registry. It pushes to an existing registry
host in a project, for example `gcr.io`. Due to this, the test images pushed to
GCR aren't cleaned up automatically at the end of the test and have to be
deleted manually. [`gcrgc`](https://github.com/graillus/gcrgc) can be used to
automatically delete all the GCR images.
```console
$ gcrgc gcr.io/<project-name>
```

#### Permissions

Following roles are needed for provisioning the infrastructure and running the
tests:
- Artifact Registry Administrator - `roles/artifactregistry.admin`
- Compute Instance Admin (v1) - `roles/compute.instanceAdmin.v1`
- Compute Storage Admin - `roles/compute.storageAdmin`
- Kubernetes Engine Admin - `roles/container.admin`
- Service Account Admin - `roles/iam.serviceAccountAdmin`
- Service Account Token Creator - `roles/iam.serviceAccountTokenCreator`
- Service Account User - `roles/iam.serviceAccountUser`
- Storage Admin - `roles/storage.admin`

#### IAM and CI setup

To create the necessary IAM role with all the permissions, set up CI secrets and
variables using
[gcp-gh-actions](https://github.com/fluxcd/test-infra/tree/main/tf-modules/gcp/github-actions)
use the terraform configuration below. Please make sure all the requirements of
gcp-gh-actions are followed before running it.

**NOTE:** When running the following for a repo under an organization, set the
environment variable `GITHUB_ORGANIZATION` if setting the `owner` in the
`github` provider doesn't work.

```hcl
provider "google" {}

provider "github" {
  owner = "fluxcd"
}

module "gcp_gh_actions" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/gcp/github-actions"

  gcp_service_account_id   = "irc-e2e"
  gcp_service_account_name = "irc-e2e"
  gcp_roles = [
    "roles/artifactregistry.admin",
    "roles/compute.instanceAdmin.v1",
    "roles/compute.storageAdmin",
    "roles/container.admin",
    "roles/iam.serviceAccountAdmin",
    "roles/iam.serviceAccountTokenCreator",
    "roles/iam.serviceAccountUser",
    "roles/storage.admin"
  ]

  github_project = "image-reflector-controller"

  github_secret_credentials_name = "IRC_E2E_GOOGLE_CREDENTIALS"
}
```

**NOTE:** The environment variables used above are for the GitHub workflow that
runs the tests. Change the variable names if needed accordingly.

## Test setup

Copy `.env.sample` to `.env`, put the respective provider configurations in the
environment variables and source it, `source .env`.

Ensure the image-reflector-controller container image to be tested is built and
ready for testing. A development image can be built from the root of the project
by running the make target `docker-build`. Or, a release image can also be
downloaded and used for testing.

Run the test with `make test-*`, setting the image-reflector image, built or
downloaded, with variable `TEST_IMG`:

```console
$ make test-aws TEST_IMG=foo/image-reflector-controller:dev
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
2022/06/15 01:55:21 pushing flux test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-test-image-reflector-direct-elephant:test
2022/06/15 01:55:41 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.0
2022/06/15 01:55:45 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.2
2022/06/15 01:55:48 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.3
2022/06/15 01:55:51 pushing test image foo111.dkr.ecr.us-east-2.amazonaws.com/flux-image-automation-test:v0.1.4
2022/06/15 01:55:54 setting images: [fluxcd/image-reflector-controller=foo111.dkr.ecr.us-east-2.amazonaws.com/flux-test-image-reflector-direct-elephant:test]
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
by setting the `TEST_IMG` variable when running the test. The kustomization is
built and the resulting flux installation manifest is written to
`build/flux.yaml`.  This is used by the test to install flux.

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

If not configured explicitly to retain the infrastructure, at the end of the
test, the test infrastructure is deleted. In case of any failure due to which
the resources don't get deleted, the `make destroy-*` commands can be run for
the respective provider. This will run terraform destroy in the respective
provider's terraform configuration directory. This can be used to quickly
destroy the infrastructure without going through the provision-test-destroy
steps.

## Debugging the tests

For debugging environment provisioning, enable verbose output with `-verbose`
test flag.

```console
$ make test-aws GO_TEST_ARGS="-verbose" TEST_IMG=foo/image-reflector-controller:dev
```

The test environment is destroyed at the end by default. Run the tests with
`-retain` flag to retain the created test infrastructure.

```console
$ make test-aws GO_TEST_ARGS="-retain" TEST_IMG=foo/image-reflector-controller:dev
```

The tests require the infrastructure state to be clean. For re-running the tests
with a retained infrastructure, set `-existing` flag.

```console
$ make test-aws GO_TEST_ARGS="-retain -existing" TEST_IMG=foo/image-reflector-controller:dev
```

To delete an existing infrastructure created with `-retain` flag:

```console
$ make test-aws GO_TEST_ARGS="-existing" TEST_IMG=foo/image-reflector-controller:dev
```
