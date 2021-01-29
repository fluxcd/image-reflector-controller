<!-- -*- fill-column: 100 -*- -->
# Image Policies

The `ImagePolicy` type gives rules for selecting a "latest" image from a scanned
`ImageRepository`. This can be used to drive automation, as with the
[image-automation-controller][];
or more generally, to inform other processes of the state of an
image repository.

## Specification

```go
// ImagePolicySpec defines the parameters for calculating the
// ImagePolicy
type ImagePolicySpec struct {
	// ImageRepositoryRef points at the object specifying the image
	// being scanned
	// +required
	ImageRepositoryRef corev1.LocalObjectReference `json:"imageRepositoryRef"`
	// Policy gives the particulars of the policy to be followed in
	// selecting the most recent image
	// +required
	Policy ImagePolicyChoice `json:"policy"`
	// FilterTags enables filtering for only a subset of tags based on a set of
	// rules. If no rules are provided, all the tags from the repository will be
	// ordered and compared.
	// +optional
	FilterTags *TagFilter `json:"filterTags,omitempty"`
}
```

The field `ImageRepositoryRef` names an `ImageRepository` object in the same namespace. It is this
object that provides the scanned image metadata for the policy to use in selecting an image.

### Policy

The ImagePolicy field specifies how to choose a latest image given the image metadata. The choice is
between

 - **SemVer**: interpreting all tags as semver versions, and choosing the highest version available
   that fits the given [semver constraints][semver-range]; or,
 - **Alphabetical**: choosing the _last_ tag when all the tags are sorted alphabetically (in either
   ascending or descending order).

```go
// ImagePolicyChoice is a union of all the types of policy that can be
// supplied.
type ImagePolicyChoice struct {
	// SemVer gives a semantic version range to check against the tags
	// available.
	// +optional
	SemVer *SemVerPolicy `json:"semver,omitempty"`
	// Alphabetical set of rules to use for alphabetical ordering of the tags.
	// +optional
	Alphabetical *AlphabeticalPolicy `json:"alphabetical,omitempty"`
}

// SemVerPolicy specifices a semantic version policy.
type SemVerPolicy struct {
	// Range gives a semver range for the image tag; the highest
	// version within the range that's a tag yields the latest image.
	// +required
	Range string `json:"range"`
}

// AlphabeticalPolicy specifices a alphabetical ordering policy.
type AlphabeticalPolicy struct {
	// Order specifies the sorting order of the tags. Given the letters of the
	// alphabet as tags, ascending order would select Z, and descending order
	// would select A.
	// +kubebuilder:default:="asc"
	// +kubebuilder:validation:Enum=asc;desc
	// +optional
	Order string `json:"order,omitempty"`
}
```

### FilterTags

```go
// TagFilter enables filtering tags based on a set of defined rules
type TagFilter struct {
	// Pattern specifies a regular expression pattern used to filter for image
	// tags.
	// +optional
	Pattern string `json:"pattern"`
	// Extract allows a capture group to be extracted from the specified regular
	// expression pattern, useful before tag evaluation.
	// +optional
	Extract string `json:"extract"`
}
```

The `FilterTags` field gives you the opportunity to filter the image tags _before_ they are
considered by the policy rule.

The `Pattern` field takes a [regular expression][regex-go] which can match anywhere in the tag string.
Only tags that match the pattern are considered by the policy rule.

The optional `Extract` value will be expanded for each tag that matches the pattern. The resulting
values will be supplied to the policy rule instead of the original tags. If `Extract` is empty, then
the tags that match the pattern will be used as they are.

## Status

```go
// ImagePolicyStatus defines the observed state of ImagePolicy
type ImagePolicyStatus struct {
	// LatestImage gives the first in the list of images scanned by
	// the image repository, when filtered and ordered according to
	// the policy.
	LatestImage string `json:"latestImage,omitempty"`
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

The `LatestImage` field contains the image selected by the policy rule, when it has run sucessfully.

### Conditions

There is one condition that may be present: the GitOps toolkit-standard `ReadyCondition`. This will
be marked as true when the policy rule has selected an image.

## Examples

Select the latest `main` branch build tagged as `${GIT_BRANCH}-${GIT_SHA:0:7}-$(date +%s)` (alphabetical):

```yaml
kind: ImagePolicy
spec:
  filterTags:
    pattern: '^main-[a-fA-F0-9]+-(?P<ts>.*)'
    extract: '$ts'
  policy:
    alphabetical:
      order: asc
```

A more strict filter would be `^main-[a-fA-F0-9]+-(?P<ts>[1-9][0-9]*)`.
Before applying policies in-cluster, you can validate your filters using
a [Go regular expression tester](https://regoio.herokuapp.com)
or [regex101.com](https://regex101.com/).

Select the latest stable version (semver):

```yaml
kind: ImagePolicy
spec:
  policy:
    semver:
      range: '>=1.0.0'
```

Select the latest stable patch version in the 1.x range (semver):

```yaml
kind: ImagePolicy
spec:
  policy:
    semver:
      range: '>=1.0.0 <2.0.0'
```

Select the latest version including pre-releases (semver):

```yaml
kind: ImagePolicy
spec:
  policy:
    semver:
      range: '>=1.0.0-0'
```

Select the latest release candidate (semver):

```yaml
kind: ImagePolicy
spec:
  filterTags:
   pattern: '.*-rc.*'
  policy:
    semver:
      range: '^1.x-0'
```

Select the latest release tagged as `RELEASE.<RFC3339-TIMESTAMP>`
e.g. [Minio](https://hub.docker.com/r/minio/minio) (alphabetical):

```yaml
kind: ImagePolicy
spec:
  filterTags:
    pattern: '^RELEASE\.(?P<timestamp>.*)Z$'
    extract: '$timestamp'
  policy:
    alphabetical:
      order: asc
```

[image-automation-controller]: https://github.com/image-automation-controller
[semver-range]: https://github.com/Masterminds/semver#checking-version-constraints
[regex-go]: https://golang.org/pkg/regexp/syntax
