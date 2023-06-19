# Image Repositories

<!-- menuweight:30 -->

The `ImageRepository` API defines a repository to scan and store a specific set
of tags in a database.

## Example

The following is an example of an ImageRepository. It scans the specified image
repository and stores the scanned tags in an internal database.

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: podinfo
  namespace: default
spec:
  image: stefanprodan/podinfo
  interval: 1h
  provider: generic
```

In the above example:

- An ImageRepository named `podinfo` is created, indicated by the
  `.metadata.name` field.
- The image-reflector-controller scans the image repository for tags every hour,
  indicated by the `.spec.interval` field.
- The registry authentication is done using a generic provider, indicated by the
  `.spec.provider` field and referenced using `.spec.secretRef`. No
  authentication is attempted when secret reference is not provided for generic
  provider. See [Provider](#provider) for more details related to registry
  authentication.
- The canonical form of the image set in `.spec.image` is used to scan the
  repository. The resolved canonical form of the image is reported in the
  `.status.canonicalImageName` field.
- The result of the scan is reported in the `.status.lastScanResult` field.

This example can be run by saving the manifest into `imagerepository.yaml`.

1. Apply the resource on the cluster:

```sh
kubectl apply -f imagerepository.yaml
```

2. Run `kubectl get imagerepository` to see the ImageRepository:

```console
NAME      LAST SCAN              TAGS
podinfo   2022-09-15T22:34:05Z   211
```

3. Run `kubectl describe imagerepository podinfo` to see the [Last Scan Result](#last-scan-result)
and [Conditions](#conditions) in the ImageRepository's Status:

```console

...
Status:
  Canonical Image Name:  index.docker.io/stefanprodan/podinfo
  Conditions:
    Last Transition Time:  2022-09-15T22:38:42Z
    Message:               successful scan, found 211 tags
    Observed Generation:   1
    Reason:                Succeeded
    Status:                True
    Type:                  Ready
  Last Scan Result:
    Latest Tags:
      latest
      6.2.0
      6.1.8
      6.1.7
      6.1.6
      6.1.5
      6.1.4
      6.1.3
      6.1.2
      6.1.1
    Scan Time:    2022-09-15T22:38:42Z
    Tag Count:    211
  Observed Exclusion List:
    ^.*\.sig$
  Observed Generation:  1
Events:
  Type    Reason     Age   From                        Message
  ----    ------     ----  ----                        -------
  Normal  Succeeded  17s   image-reflector-controller  successful scan, found 211 tags
```

## Writing an ImageRepository spec

As with all other Kubernetes config, an ImageRepository needs `apiVersion`,
`kind`, and `metadata` fields. The name of an ImageRepository object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).

An ImageRepository also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Image

`.spec.image` is a required field that specifies the address of an image
repository without any scheme prefix, e.g. `fluxcd/image-reflector-controller`.
This image is converted to its canonical form by the controller before scanning.
The canonical form of the image is reflected in `.status.canonicalImageName`.

### Interval

`.spec.interval` is a required field that specifies the interval at which the
Image repository must be scanned.

After successfully reconciling the object, the image-reflector-controller
requeues it for inspection after the specified interval. The value must be in a
[Go recognized duration string format](https://pkg.go.dev/time#ParseDuration),
e.g. `10m0s` to reconcile the object every 10 minutes.

If the `.metadata.generation` of a resource changes (due to e.g. a change to
the spec), this is handled instantly outside the interval window.

### Timeout

`.spec.timeout` is an optional field to specify a timeout for various operations
during the reconciliation like fetching the referred secrets, scanning the
repository, etc. The value must be in a
[Go recognized duration string format](https://pkg.go.dev/time#ParseDuration),
e.g. `1m30s` for a timeout of one minute and thirty seconds. The default value
is the value of `.spec.interval`.

### Secret reference

`.spec.secretRef.name` is an optional field to specify a name reference to a
Secret in the same namespace as the ImageRepository, containing authentication
credentials for the Image repository. The secret is expected to be in the same
format as the [docker config secrets](https://kubernetes.io/docs/concepts/configuration/secret/#docker-config-secrets), usually created by `kubectl create secret
docker-registry`.

Example of using secret reference in an ImageRepository:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: podinfo
  namespace: default
spec:
  image: stefanprodan/podinfo
  interval: 1h
  secretRef:
    name: regcred
---
apiVersion: v1
kind: Secret
metadata:
  name: regcred
  namespace: default
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eyJhdXRocyI6eyJodHRwczovL2luZGV4LmRvY2tlci5pby92MS8iOnsidXNlcm5hbWUiOiJmb28iLCJwYXNzd29yZCI6ImJhciIsImF1dGgiOiJabTl2T21KaGNnPT0ifX19
```

For a publicly accessible image repository, there's no need to provide a secret
reference.

### ServiceAccount name

`.spec.serviceAccountName` is an optional field to specify a name reference to a
ServiceAccount in the same namespace as the ImageRepository, with an image pull
secret attached to it. For detailed instructions about attaching an image pull
secret to a ServiceAccount, see [Add image pull secret to service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-image-pull-secret-to-service-account).

### Certificate secret reference

`.spec.certSecretRef.name` is an optional field to specify a secret containing
TLS certificate data. The secret can contain the following keys:

* `tls.crt` and `tls.key`, to specify the client certificate and private key used
for TLS client authentication. These must be used in conjunction, i.e.
specifying one without the other will lead to an error.
* `ca.crt`, to specify the CA certificate used to verify the server, which is
required if the server is using a self-signed certificate.

If the server is using a self-signed certificate and has TLS client
authentication enabled, all three values are required.

The Secret should be of type `Opaque` or `kubernetes.io/tls`. All the files in
the Secret are expected to be [PEM-encoded][pem-encoding]. Assuming you have
three files; `client.key`, `client.crt` and `ca.crt` for the client private key,
client certificate and the CA certificate respectively, you can generate the
required Secret using the `flux create secret tls` command:

```sh
flux create secret tls --tls-key-file=client.key --tls-crt-file=client.crt --ca-crt-file=ca.crt
```

Example usage:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: example
  namespace: default
spec:
  interval: 5m0s
  url: example.com
  certSecretRef:
    name: example-tls
---
apiVersion: v1
kind: Secret
metadata:
  name: example-tls
  namespace: default
type: kubernetes.io/tls # or Opaque
data:
  tls.crt: <BASE64>
  tls.key: <BASE64>
  # NOTE: Can be supplied without the above values
  ca.crt: <BASE64>
```

**Warning:** Support for the `caFile`, `certFile` and `keyFile` keys have been
deprecated. If you have any Secrets using these keys and specified in an
ImageRepository, the controller will log a deprecation warning.

### Suspend

`.spec.suspend` is an optional field to suspend the reconciliation of an
ImageRepository. When set to `true`, the controller will stop reconciling the
ImageRepository, and changes to the resource or image repository will not result
in new scan results. When the field is set to `false` or removed, it will
resume.

### Access from

`.spec.accessFrom` is an optional field to restrict cross-namespace access of
ImageRepositories. To grant access to an ImageRepository for policies in other
namespaces, the owner of the ImageRepository has to specify a list of label
selectors that match the namespace labels of the ImagePolicy objects.

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: app1
  namespace: apps
spec:
  interval: 1h
  image: docker.io/org/image
  secretRef:
    name: regcred
  accessFrom:
    namespaceSelectors:
      - matchLabels:
          kubernetes.io/metadata.name: flux-system
```

**Note:** The `kubernetes.io/metadata.name` label above is a readonly label
added by Kubernetes >= 1.21 automatically on namespaces. For older version of
Kubernetes, please set labels on the namespaces where the ImagePolicy exist.

The above definition, allows ImagePolicy in the `flux-system` namespace to
reference the `app1` ImageRepository e.g.:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: app1
  namespace: flux-system
spec:
  imageRepositoryRef:
    name: app1
    namespace: apps
  policy:
    semver:
      range: 1.0.x
```

To grant access to all namespaces, an empty `matchLabels` can be set:

```yaml
  accessFrom:
    namespaceSelectors:
      - matchLabels: {}
```

### Exclusion list

`.spec.exclusionList` is an optional field to exclude certain tags in the image
scan result. It's a list of regular expression patterns with a default value of
`"^.*\\.sig$"` if it's not set. This default value is used to exclude all the
tags ending with `.sig`, since these are [Cosign](https://github.com/sigstore/cosign)
generated objects and not container images which can be deployed on a Kubernetes
cluster.

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: app1
  namespace: apps
spec:
  interval: 1h
  image: docker.io/org/image
  exclusionList:
    - "^.*\\.sig$"
    - "1.0.2"
    - "1.1.1|1.0.0"
```

### Insecure

`.spec.insecure` is an optional field to specify that the image registry is
hosted at a non-TLS endpoint and thus the controller should use plain HTTP
requests to communicate with the registry.

> If an ImageRepository has `.spec.insecure` as `true` and the controller has
  `--insecure-allow-http` set to `false`, then the object is marked as stalled.
  For more details, see: https://github.com/fluxcd/flux2/tree/ddcc301ab6289e0640174cb9f3d46f1eeab57927/rfcs/0004-insecure-http#design-details

### Provider

`.spec.provider` is an optional field that allows specifying an OIDC provider
used for authentication purposes.

Supported options are:

- `generic`
- `aws`
- `azure`
- `gcp`

The `generic` provider can be used for public repositories or when static
credentials are used for authentication, either with `.spec.secretRef` or
`.spec.serviceAccount`. If `.spec.provider` is not specified, it defaults to
`generic`.

#### AWS

The `aws` provider can be used to authenticate automatically using the EKS
worker node IAM role or IAM Role for Service Accounts (IRSA), and by extension
gain access to ECR.

##### Worker Node IAM

When the worker node IAM role has access to ECR, image-reflector-controller
running on it will also have access to ECR. Please take a look at this
[documentation](https://docs.aws.amazon.com/eks/latest/userguide/create-node-role.html)
for creating worker node IAM roles.

##### IAM roles for service accounts(IRSA)

When using IRSA to enable access to ECR, add the following patch to your
bootstrap repository, in the `flux-system/kustomization.yaml` file:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - gotk-components.yaml
  - gotk-sync.yaml
patches:
  - patch: |
      apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: image-reflector-controller
        annotations:
          eks.amazonaws.com/role-arn: <role arn>
    target:
      kind: ServiceAccount
      name: image-reflector-controller
```

Note that you can attach the AWS managed policy `arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly`
to the IAM role when using IRSA and you have to configure the 
`image-reflector-controller` to assume the IAM role. Please see 
[documentation](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html).

#### Azure

The `azure` provider can be used to authenticate automatically using Workload
Identity, kubelet managed identity or Azure Active Directory pod-managed 
identity (aad-pod-identity), and by extension gain access to ACR.

##### Kubelet Identity

When the kubelet managed identity has access to ACR, image-reflector-controller
running on it will also have access to ACR.

##### Workload Identity

When using workload identity to enable access to ACR, add the following patch to
properly annotate the image-reflector-controller pods and service account 
in the `flux-system/kustomization.yaml` file:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - gotk-components.yaml
  - gotk-sync.yaml
patches:
  - patch: |-
      apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: image-reflector-controller
        namespace: flux-system
        annotations:
          azure.workload.identity/client-id: <AZURE_CLIENT_ID>
        labels:
          azure.workload.identity/use: "true"
  - patch: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: image-reflector-controller
        namespace: flux-system
        labels:
          azure.workload.identity/use: "true"
      spec:
        template:
          metadata:
            labels:
              azure.workload.identity/use: "true"
```

To use workload identity on your cluster, you would have to install workload
in your cluster, create an identity that has `AcrPull` role to ACR and establish 
azure federated identity between the identity and the image-reflector-controller
service account. Please, take a look at the
[Azure documentation for Workload identity](https://azure.github.io/azure-workload-identity/docs/quick-start.html).

##### AAD Pod Identity

When using aad-pod-identity to enable access to ACR, add the following patch to
your bootstrap repository, in the `flux-system/kustomization.yaml` file:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - gotk-components.yaml
  - gotk-sync.yaml
patches:
  - patch: |
      - op: add
        path: /spec/template/metadata/labels/aadpodidbinding
        value: <identity-name>
    target:
      kind: Deployment
      name: image-reflector-controller
```

When using pod-managed identity on an AKS cluster, AAD Pod Identity
has to be used to give the `image-reflector-controller` pod access to the ACR.
To do this, you have to install `aad-pod-identity` on your cluster, create a
managed identity that has access to the container registry (this can also be the
Kubelet identity if it has `AcrPull` role assignment on the ACR), create an
`AzureIdentity` and `AzureIdentityBinding` that describe the managed identity
and then label the `image-reflector-controller` pods with the name of the
AzureIdentity as shown in the patch above. Please take a look at
[this guide](https://azure.github.io/aad-pod-identity/docs/) or
[this one](https://docs.microsoft.com/en-us/azure/aks/use-azure-ad-pod-identity)
to use AKS pod-managed identities add-on that is in preview.

#### GCP

The `gcp` provider can be used to authenticate automatically using OAuth scopes
or Workload Identity, and by extension gain access to GCR or Artifact Registry.

##### Access scopes

When the GKE nodes have the appropriate OAuth scope for accessing GCR and
Artifact Registry, image-reflector-controller running on it will also have
access to them.

##### Workload Identity

When using Workload Identity to enable access to GCR or Artifact Registry, add
the following patch to your bootstrap repository, in the
`flux-system/kustomization.yaml` file:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - gotk-components.yaml
  - gotk-sync.yaml
patches:
  - patch: |
      apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: image-reflector-controller
        annotations:
          iam.gke.io/gcp-service-account: <identity-name>
    target:
      kind: ServiceAccount
      name: image-reflector-controller
```

The Artifact Registry service uses the permission `artifactregistry.repositories.downloadArtifacts`
that is located under the Artifact Registry Reader role. If you are using
Google Container Registry service, the needed permission is instead `storage.objects.list`
which can be bound as part of the Container Registry Service Agent role.
Take a look at [this guide](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
for more information about setting up GKE Workload Identity.

#### Authentication on other platforms

For other platforms that link service permissions to service accounts, secret
can be created using tooling for that platform, rather than directly with
`kubectl create secret`. There is advice specific to some platforms in [the
image automation guide][image-auto-provider-secrets].

## Working with ImageRepositories

### Triggering a reconcile

To manually tell the image-reflector-controller to reconcile an ImageRepository
outside the [specified interval window](#interval), an ImageRepository can be
annotated with `reconcile.fluxcd.io/requestedAt: <arbitrary value>`. Annotating
the resource queues the ImageRepository for reconciliation if the
`<arbitrary-value>` differs from the last value the controller acted on, as
reported in [`.status.lastHandledReconcileAt`](#last-handled-reconcile-at).

Using `kubectl`:

```sh
kubectl annotate --field-manager=flux-client-side-apply --overwrite imagerepository/<repository-name> reconcile.fluxcd.io/requestedAt="$(date +%s)"
```

Using `flux`:

```sh
flux reconcile image repository <repository-name>
```

### Waiting for `Ready`

When a change is applied, it is possible to wait for the ImageRepository to
reach a [ready state](#ready-imagerepository) using `kubectl`:

```sh
kubectl wait imagerepository/<repository-name> --for=condition=ready --timeout=1m
```

### Suspending and resuming

When you find yourself in a situation where you temporarily want to pause the
reconciliation of a ImageRepository, you can suspend it using the
[`.spec.suspend` field](#suspend).

#### Suspend an ImageRepository

In your YAML declaration:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: <repository-name>
spec:
  suspend: true
```

Using `kubectl`:

```sh
kubectl patch imagerepository <repository-name> --field-manager=flux-client-side-apply -p '{\"spec\": {\"suspend\" : true }}'
```

Using `flux`:

```sh
flux suspend image repository <repository-name>
```

**Note:** When an ImageRepository has scan results and is suspended, and this
result later disappears from the database due to e.g. the
image-reflector-controller Pod being evicted from a Node, this will not be
reflected in the ImageRepository's Status until it is resumed.

#### Resume an ImageRepository

In your YAML declaration, comment out (or remove) the `.spec.suspend` field:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: <repository-name>
spec:
  # suspend: true
```

**Note:** Setting the field value to `false` has the same effect as removing
it, but does not allow for "hot patching" using e.g. `kubectl` while practicing
GitOps; as the manually applied patch would be overwritten by the declared
state in Git.

Using `kubectl`:

```sh
kubectl patch imagerepository <repository-name> --field-manager=flux-client-side-apply -p '{\"spec\" : {\"suspend\" : false }}'
```

Using `flux`:

```sh
flux resume image repository <repository-name>
```

### Debugging an ImageRepository

There are several ways to gather information about an ImageRepository for
debugging purposes.

#### Describe the ImageRepository

Describing an ImageRepository using
`kubectl describe imagerepository <repository-name>`
displays the latest recorded information for the resource in the `Status` and
`Events` sections:

```console
...
Status:
  Conditions:
    Last Transition Time:  2022-09-19T05:47:40Z
    Message:               could not parse reference: ghcr.io/stefanprodan/podinfo:foo:bar
    Observed Generation:   1
    Reason:                ImageURLInvalid
    Status:                True
    Type:                  Stalled
    Last Transition Time:  2022-09-19T05:47:40Z
    Message:               could not parse reference: ghcr.io/stefanprodan/podinfo:foo:bar
    Observed Generation:   1
    Reason:                ImageURLInvalid
    Status:                False
    Type:                  Ready
  Observed Generation:     1
Events:
  Type     Reason           Age   From                        Message
  ----     ------           ----  ----                        -------
  Warning  ImageURLInvalid  5s    image-reflector-controller  could not parse reference: ghcr.io/stefanprodan/podinfo:foo:bar
```

#### Trace emitted Events

To view events for specific ImageRepository(s), `kubectl events` can be used
in combination with `--for` to list the Events for specific objects. For
example, running

```sh
kubectl events --for ImageRepository/<repository-name>
```

lists

```console
LAST SEEN   TYPE      REASON            OBJECT                              MESSAGE
3m51s       Normal    Succeeded         imagerepository/<repository-name>   successful scan, found 34 tags
114s        Warning   ImageURLInvalid   imagerepository/<repository-name>   could not parse reference: ghcr.io/stefanprodan/podinfo:foo:bar
```

Besides being reported in Events, the reconciliation errors are also logged by
the controller. The Flux CLI offer commands for filtering the logs for a
specific ImageRepository, e.g.
`flux logs --level=error --kind=ImageRepository --name=<repository-name>`.

## ImageRepository Status

### Last Scan Result

The ImageRepository reports the latest scanned tags from the image repository in
`.status.lastScanResult` for the resource. The tags are stored in an internal
database. `.status.lastScanResult.scanTime` shows the time of last scan.
`.status.lastScanResult.tagCount` shows the number of tags in the result. This
is calculated after applying any exclusion list rules.

Example:
```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: <repository-name>
status:
  lastScanResult:
    latestTags:
    - latest
    - 6.2.0
    - 6.1.8
    - 6.1.7
    - 6.1.6
    - 6.1.5
    - 6.1.4
    - 6.1.3
    - 6.1.2
    - 6.1.1
    scanTime: "2022-09-19T05:53:27Z"
    tagCount: 34
```

### Canonical Image Name

The ImageRepository reports the canonical form of the image repository provided
in the ImageRepository's `.spec.image` in `.status.canonicalImageName`.
Canonical name is the name of the image repository with all the implied bits
made explicit; e.g., `docker.io/library/alpine` rather than `alpine`.

### Observed Exclusion List

The ImageRepository reports an observed exclusion list in the ImageRepository's
`.status.observedExclusionList`. The observed exclusion list is the latest
`.spec.exclusionList` which resulted in a [ready state](#ready-imagerepository),
or stalled due to error it can not recover from without human intervention.

### Conditions

An ImageRepository enters various states during its lifecycle, reflected as
[Kubernetes Conditions][typical-status-properties].
It can be [reconciling](#reconciling-imagerepository) while scanning the image
repository, it can be [ready](#ready-imagerepository), or it can [fail during
reconciliation](#failed-imagerepository).

The ImageRepository API is compatible with the [kstatus specification][kstatus-spec],
and reports `Reconciling` and `Stalled` conditions where applicable to provide
better (timeout) support to solutions polling the ImageRepository to become
`Ready`.

#### Reconciling ImageRepository

The image-reflector-controller marks an ImageRepository as _reconciling_ when
one of the following is true:

- The generation of the ImageRepository is newer than the [Observed
Generation](#observed-generation).
- The ImageRepository is being scanned because it's scan time as per the
  specified `spec.interval`, or the ImageRepository has never been scanned
  before, or the reported tags in the last scanned results have disappeared
  from the database.

When the ImageRepository is "reconciling", the `Ready` Condition status becomes
`False`, and the controller adds a Condition with the following attributes to
the ImageRepository's `.status.conditions`:

- `type: Reconciling`
- `status: "True"`
- `reason: NewGeneration` | `reason: Scanning`

It has a ["negative polarity"][typical-status-properties], and is only present
on the ImageRepository while its status value is `"True"`.

#### Ready ImageRepository

The image-reflector-controller marks an ImageRepository as _ready_ when it has
the following characteristics:

- The ImageRepository reports a [Last Scan Result](#last-scan-result).
- The reported tags exists in the controller's internal database.
- The controller was able to communicate with the remote image repository using
  the current spec.

When the ImageRepository is "ready", the controller sets a Condition with the
following attributes in the ImageRepository's `.status.conditions`:

- `type: Ready`
- `status: "True"`
- `reason: Succeeded`

This `Ready` Condition will retain a status value of `"True"` until the
ImageRepository is marked as [reconciling](#reconciling-imagerepository), or
e.g. a [transient error](#failed-imagerepository) occurs due to a temporary
network issue.

#### Failed ImageRepository

The image-reflector-controller may get stuck trying to scan an image repository
without completing. This can occur due to some of the following factors:

- The remote image repository is temporarily unavailable.
- The image repository does not exist.
- The [Secret reference](#secret-reference) and [Certificate secret reference](#certificate-secret-reference)
  contains a reference to a non-existing Secret.
- The credentials and certificate in the referenced Secret are invalid.
- The ImageRepository spec contains a generic misconfiguration.
- A database related failure when reading or writing the scanned tags.

When this happens, the controller sets the `Ready` Condition status to `False`
with the following reasons:

- `reason: ImageURLInvalid` | `reason: AuthenticationFailed` | `reason: Failure` | `reason: ReadOperationFailed`

While the ImageRepository is in failing state, the controller will continue to
attempt to scan the image repository for the resource with an exponential
backoff, until it succeeds and the ImageRepository is marked as
[ready](#ready-imagerepository).

Note that an ImageRepository can be [reconciling](#reconciling-imagerepository)
while failing at the same time, for example due to a newly introduced
configuration issue in the ImageRepository spec.

### Observed Generation

The image-reflector-controller reports an
[observed generation][typical-status-properties] in the ImageRepository's
`.status.observedGeneration`. The observed generation is the latest
`.metadata.generation` which resulted in either a
[ready state](#ready-imagerepository), or stalled due to error it can not
recover from without human intervention.

### Last Handled Reconcile At

The image-reflector-controller reports the last
`reconcile.fluxcd.io/requestedAt` annotation value it acted on in the
`.status.lastHandledReconcileAt` field.

For practical information about this field, see [triggering a
reconcile](#triggering-a-reconcile).

[image-auto-provider-secrets]: https://fluxcd.io/flux/guides/image-update/#imagerepository-cloud-providers-authentication
[pem-encoding]: https://en.wikipedia.org/wiki/Privacy-Enhanced_Mail
[sops-guide]: https://fluxcd.io/flux/guides/mozilla-sops/
[cloud providers authentication guide]: https://fluxcd.io/flux/guides/image-update/#imagerepository-cloud-providers-authentication
[typical-status-properties]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
[kstatus-spec]: https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
