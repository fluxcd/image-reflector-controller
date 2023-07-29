# Image Policies

<!-- menuweight:20 -->

The `ImagePolicies` API defines rules for selecting a "latest" image from
`ImageRepositories`.

## Example

The following is an example of an ImagePolicy. It queries the referred
ImageRepository for the image name of the repository, reads all the tags in
the repository and selects the latest tag based on the defined policy rules.

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: podinfo
  namespace: default
spec:
  imageRepositoryRef:
    name: podinfo
  policy:
    semver:
      range: 5.1.x
```

In the above example:

- An ImagePolicy named `podinfo` is created, indicated by the `.metadata.name`
  field.
- The image-reflector-controller applies the latest tag selection policy every
  time there's an update in the referred ImageRepository, indicated by the
  `.spec.imageRepositoryRef.name` field.
- It fetches the canonical image name of the referred ImageRepository and reads
  the scanned tags from the internal database for the image name. The read tags
  are then used to select the latest tag based on the policy defined in
  `.spec.policy`.
- The latest image is constructed with the ImageRepository image and the
  selected tag, and reported in the `.status.latestImage`.

This example can be run by saving the manifest into `imagepolicy.yaml`.

1. Apply the resource on the cluster:

```sh
kubectl apply -f imagepolicy.yaml
```

2. Run `kubectl get imagepolicy` to see the ImagePolicy:

```console
NAME      LATESTIMAGE
podinfo   ghcr.io/stefanprodan/podinfo:5.1.4
```

3. Run `kubectl describe imagepolicy podinfo` to see the [Latest Image](#latest-image)
and [Conditions](#conditions) in the ImagePolicy's Status:

```console
Status:
  Conditions:
    Last Transition Time:  2022-09-20T07:09:56Z
    Message:               Latest image tag for 'ghcr.io/stefanprodan/podinfo' resolved to 5.1.4
    Observed Generation:   1
    Reason:                Succeeded
    Status:                True
    Type:                  Ready
  Latest Image:            ghcr.io/stefanprodan/podinfo:5.1.4
  Observed Generation:     1
Events:
  Type    Reason     Age              From                        Message
  ----    ------     ----             ----                        -------
  Normal  Succeeded  7s (x3 over 8s)  image-reflector-controller  Latest image tag for 'ghcr.io/stefanprodan/podinfo' resolved to 5.1.4
```

## Writing an ImagePolicy spec

As with all other Kubernetes config, an ImagePolicy needs `apiVersion`,
`kind`, and `metadata` fields. The name of an ImagePolicy object must be a
valid [DNS subdomain name](https://kubernetes.io/docs/concepts/overview/working-with-objects/names#dns-subdomain-names).

An ImagePolicy also needs a
[`.spec` section](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status).

### Image Repository Reference

`.spec.imageRepositoryRef` is a required field that specifies the
ImageRepository for which the latest image has to be selected. The value must be
a namespaced object reference. For ImageRepository in the same namespace as the
ImagePolicy, no namespace needs to be provided. For ImageRepository in a
different namespace than the namespace of the ImagePolicy, namespace name has to
be provided. For example:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: podinfo
  namespace: default
spec:
  imageRepositoryRef:
    name: podinfo
    namespace: flux-system
...
```

The ImageRepository access is determied by its ACL for cross-namespace
reference. For more details on how to allow cross-namespace references see the
[ImageRepository docs](imagerepositories.md#access-from).

### Policy

`.spec.policy` is a required field that specifies how to choose a latest image
given the image metadata. There are three image policy choices:
- SemVer
- Alphabetical
- Numerical

#### SemVer

SemVer policy interprets all the tags as semver versions and chooses the highest
version available that fits the given
[semver constraints](https://github.com/Masterminds/semver#checking-version-constraints).
The constraint is set in the `.spec.policy.semver.range` field.

Example of a SemVer image policy choice:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: podinfo
spec:
  imageRepositoryRef:
    name: podinfo
  policy:
    semver:
      range: '>=1.0.0'
```

This will select the latest stable version tag.

#### Alphabetical

Alphabetical policy chooses the _last_ tag when all the tags are sorted
alphabetically (in either ascending or descending order). The sort order is set
in the `.spec.policy.alphabetical.order` field. The value could be `asc` for
ascending order or `desc` for descending order. The default value is `asc`.

Example of an Alphabetical policy choice:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: podinfo
spec:
  imageRepositoryRef:
    name: podinfo
  policy:
    alphabetical:
      order: asc
```

This will select the last tag when all the tags are sorted alphabetically in
ascending order.

#### Numerical

Numerical policy chooses the _last_ tag when all the tags are sorted numerically
(in either ascending or descending order). The sort order is set in the
`.spec.policy.numerical.order` field. The value could be `asc` for ascending
order or `desc` for descending order. The default value is `asc`.

Example of a Numerical policy choice:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: podinfo
spec:
  imageRepositoryRef:
    name: podinfo
  policy:
    numerical:
      order: asc
```

This will select the last tag when all the tags are sorted numerically in
ascending order.

### Filter Tags

`.spec.filterTags` is an optional field to specify a filter on the image tags
before they are considered by the policy rule.

The filter pattern is a regular expression, set in the
`.spec.filterTags.pattern` field. Only tags that match the pattern are
considered by the policy rule.

The `.spec.filterTags.extract` is an optional field used to extract a value from
the matching tags which is supplied to the policy rule instead of the original
tags. If unspecified, the tags that match the pattern will be used as they are.

Example of selecting the latest release candidate (semver):

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: podinfo
spec:
  imageRepositoryRef:
    name: podinfo
  filterTags:
    pattern: '.*-rc.*'
  policy:
    semver:
      range: '^1.x-0'
```

Example of selecting the latest release tagged as `RELEASE.<RFC3339-TIMESTAMP>`
(alphabetical):

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: minio
spec:
  imageRepositoryRef:
    name: minio
  filterTags:
    pattern: '^RELEASE\.(?P<timestamp>.*)Z$'
    extract: '$timestamp'
  policy:
    alphabetical:
      order: asc
```

In the above example, the timestamp value from the tag pattern is extracted and
used in the policy rule to determine the latest tag.

## Working with ImagePolicy

### Triggering a reconcile

ImagePolicy is reconciled automatically when the associated ImageRepository is
updated. Whenever ImageRepository gets updated, ImagePolicy will be triggered
and have the policy result based on the latest values of ImageRepository. To
manually tell the image-reflector-controller to reconcile an ImagePolicy, the
associated ImageRepository can be annotated with
`reconcile.fluxcd.io/requestedAt: <arbitrary value>`.
See [triggering a reconcile](imagerepositories.md#triggering-a-reconcile) for
more details about reconciling ImageRepository.

### Waiting for `Ready`

When a change is applied, it is possible to wait for the ImagePolicy to reach a
[ready state](#ready-imagepolicy) using `kubectl`:

```sh
kubectl wait imagepolicy/<policy-name> --for=condition=ready --timeout=1m
```

### Debugging an ImagePolicy

There are several ways to gather information about an ImagePolicy for debugging
purposes.

#### Describe the ImagePolicy

Describing an ImagePolicy using `kubectl describe imagepolicy <policy-name>`
displays the latest recorded information for the resource in the `Status` and
`Events` sections:

```console
...
Status:
  Conditions:
    Last Transition Time:  2022-10-06T12:07:35Z
    Message:               accessing ImageRepository
    Observed Generation:   1
    Reason:                AccessingRepository
    Status:                True
    Type:                  Reconciling
    Last Transition Time:  2022-10-06T12:07:35Z
    Message:               failed to get the referred ImageRepository: referenced ImageRepository does not exist: ImageRepository.image.toolkit.fluxcd.io "podinfo" not found
    Observed Generation:   1
    Reason:                DependencyNotReady
    Status:                False
    Type:                  Ready
  Observed Generation:     1
Events:
  Type     Reason              Age                From                        Message
  ----     ------              ----               ----                        -------
  Warning  DependencyNotReady  2s (x4 over 5s)    image-reflector-controller  failed to get the referred ImageRepository: referenced ImageRepository does not exist: ImageRepository.image.toolkit.fluxcd.io "podinfo" not found
```

#### Trace emitted Events

To view events for specific ImagePolicy(s), `kubectl events` can be used in
combination with `--for` to list the Events for specific objects. For example,
running

```sh
kubectl events --for ImagePolicy/<policy-name>
```

lists

```console
LAST SEEN   TYPE      REASON               OBJECT                      MESSAGE
4m44s       Normal    Succeeded            imagepolicy/<policy-name>   Latest image tag for 'ghcr.io/stefanprodan/podinfo' resolved to 5.1.4
95s         Warning   DependencyNotReady   imagepolicy/<policy-name>   failed to get the referred ImageRepository: referenced ImageRepository does not exist: ImageRepository.image.toolkit.fluxcd.io "podinfo" not found
```

Besides being reported in Events, the reconciliation errors are also logged by
the controller. The Flux CLI offer commands for filtering the logs for a
specific ImagePolicy, e.g.
`flux logs --level=error --kind=ImagePolicy --name=<policy-name>`.

## ImagePolicy Status

### Latest Image

The ImagePolicy reports the latest select image from the ImageRepository tags in
`.status.latestImage` for the resource.

Example:

```yaml
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: <policy-name>
status:
  latestImage: ghcr.io/stefanprodan/podinfo:5.1.4
```

### Observed Previous Image

The ImagePolicy reports the previously observed latest image in
`.status.observedPreviousImage` for the resource. This is used by the
ImagePolicy to determine an upgrade path of an ImagePolicy update. This field
is reset when the ImagePolicy fails due to some reason to be able to distinguish
between a failure recovery and a genuine latest image upgrade.

Example:

```yaml
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: <policy-name>
status:
  latestImage: ghcr.io/stefanprodan/podinfo:6.2.1
  observedPreviousImage: ghcr.io/stefanprodan/podinfo:5.1.4
```

### Conditions

An ImagePolicy enters various states during its lifecycle, reflected as
[Kubernetes Conditions][typical-status-properties].
It can be [reconciling](#reconciling-imagepolicy) while reading the tags from
ImageRepository scan results, it can be [ready](#ready-imagepolicy), or it can
[fail during reconciliation](#failed-imagepolicy).

The ImagePolicy API is compatible with the [kstatus specification][kstatus-spec],
and reports `Reconciling` and `Stalled` conditions where applicable to provide
better (timeout) support to solutions polling the ImagePolicy to become `Ready`.

#### Reconciling ImagePolicy

The image-reflector-controller marks an ImagePolicy as _reconciling_ when one of
the following is true:

- The generation of the ImagePolicy is newer than the [Observed Generation](#observed-generation).
- The ImagePolicy is accessing the provided ImageRepository reference.
- The ImagePolicy is being applied to the tags read from an ImageRepository.

When the ImagePolicy is "reconciling", the `Ready` Condition status becomes
`False`, and the controller adds a Condition with the following attributes to
the ImagePolicy's `.status.conditions`:

- `type: Reconciling`
- `status: "True"`
- `reason: NewGeneration` | `reason:AccessingRepository` | `reason: ApplyingPolicy`

It has a ["negative polarity"][typical-status-properties], and is only present
on the ImagePolicy while its status value is `"True"`.

#### Ready ImagePolicy

The image-reflector-controller marks an ImagePolicy as _ready_ when it has the
following characteristics:

- The ImagePolicy reports a [Latest Image](#latest-image)
- The referenced ImageRepository is accessible and the internal tags database
  contains the tags that ImagePolicy needs to apply the policy on.

When the ImagePolicy is "ready", the controller sets a Condition with the
following attributes in the ImagePolicy's `.status.conditions`.

- `type: Ready`
- `status: "True"`
- `reason: Succeeded`

This `Ready` Condition will retain a status value of `"True"` until the
ImagePolicy is marked as [reconciling](#reconciling-imagepolicy), or e.g. a
[transient error](#failed-imagepolicy) occurs due to a temporary network issue.

#### Failed ImagePolicy

The image-reflector-controller may get stuck trying to apply a policy without
completing. This can occur due to some of the following factors:

- The referenced ImageRepository is temporarily unavailable.
- The referenced ImageRepository does not exist.
- The referenced ImageRepository is not accessible in a different namespace.
- The ImagePolicy spec contains a generic misconfiguration.
- The ImagePolicy could not select the latest tag based on the given rules and
  the available tags.
- A database related failure when reading or writing the scanned tags.

When this happens, the controller sets the `Ready` condition status to `False`
wit the following reason:

- `reason: Failure` | `reason: AccessDenied` | `reason: DependencyNotReady`

While the ImagePolicy is in failing state, the controller will continue to
attempt to get the referenced ImageRepository for the resource and apply the
policy rules with an exponential backoff, until it succeeds and the ImagePolicy
is marked as [ready](#ready-imagepolicy).

Note that an ImagePolicy can be [reconcilcing](#reconciling-imagepolicy) while
failing at the same time, for example due to a newly introduced configuration
issue in the ImagePolicy spec.

### Observed Generation

The image-reflector-controller reports an
[observed generation][typical-status-properties] in the ImagePolicy's
`.status.observedGeneration`. The observed generation is the latest
`.metadata.generation` which resulted in either a
[ready state](#ready-imagepolicy), or stalled due to error it can not
recover from without human intervention.

[typical-status-properties]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
[kstatus-spec]: https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
