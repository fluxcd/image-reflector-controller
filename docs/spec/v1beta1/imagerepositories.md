<!-- -*- fill-column: 100 -*- -->
# Image Repositories

The `ImageRepository` API specifies how to scan OCI image repositories. A _repository_ is a
collection of images -- e.g., `alpine`, as opposed to a specific image, e.g.,
`alpine:3.1`. `ImagePolicy` objects can then refer to an `ImageRepository` in order to select a
specific image from those scanned.

## Specification

The **ImageRepository** type holds details for scanning a particular image repository.

```go
// ImageRepositorySpec defines the parameters for scanning an image
// repository, e.g., `fluxcd/flux`.
type ImageRepositorySpec struct {
	// Image is the name of the image repository
	// +required
	Image string `json:"image,omitempty"`
	// Interval is the length of time to wait between
	// scans of the image repository.
	// +required
	Interval metav1.Duration `json:"interval,omitempty"`

	// Timeout for image scanning.
	// Defaults to 'Interval' duration.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// SecretRef can be given the name of a secret containing
	// credentials to use for the image registry. The secret should be
	// created with `kubectl create secret docker-registry`, or the
	// equivalent.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// CertSecretRef can be given the name of a secret containing
	// either or both of
	//
	//  - a PEM-encoded client certificate (`certFile`) and private
	//  key (`keyFile`);
	//  - a PEM-encoded CA certificate (`caFile`)
	//
	//  and whichever are supplied, will be used for connecting to the
	//  registry. The client cert and key are useful if you are
	//  authenticating with a certificate; the CA cert is useful if
	//  you are using a self-signed server certificate.
	// +optional
	CertSecretRef *meta.LocalObjectReference `json:"certSecretRef,omitempty"`

	// This flag tells the controller to suspend subsequent image scans.
	// It does not apply to already started scans. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
	
	// AccessFrom defines an ACL for allowing cross-namespace references
	// to the ImageRepository object based on the caller's namespace labels.
	// +optional
	AccessFrom *AccessFrom `json:"accessFrom,omitempty"`
}
```

The `Suspend` field can be set to `true` to stop the controller scanning the image repository
specified; remove the field value or set to `false` to resume scanning.

### Authentication

The `secretRef` names a secret in the same namespace that holds credentials for accessing the image
repository. This secret is expected to be in the same format as for
[`imagePullSecrets`][image-pull-secrets]. The usual way to create such a secret is with

    kubectl create secret docker-registry ...

For a publicly accessible image repository, you will not need to provide a `secretRef`.

#### Automatic Authentication

When running on any of the three major cloud providers and using their container registry to store images,
you should be able to rely on the controller retrieving credentials automatically. The controllers must be run
with the corresponding flag for each provider.

For  [<abbr title="Elastic Kubernetes Service">EKS</abbr>][EKS] and [<abbr title="Elastic Container Registry">ECR</abbr>][ECR], 
the flag is `--aws-autologin-for-ecr`.
For [<abbr title="Google Kubernetes Engine">GKE</abbr>][GKE] and [<abbr title="Google Container Registry">GCR</abbr>][GCR],
the flag is `--gcp-autologin-for-gcr`.
For [<abbr title="Azure Kubernetes Service">AKS</abbr>][AKS] and [<abbr title="Azure Container Registry">ACR</abbr>][ACR],
the flag is  `--azure-autologin-for-acr`.

#### Other platforms

If you are running on another platform that links service permissions to service accounts, you will
need to create the secret using tooling for that platform, rather than directly with `kubectl create
secret`. There is advice specific to some platforms [in the image automation
guide][image-auto-provider-secrets].

### TLS Certificates

The `certSecretRef` field names a secret with TLS certificate data. This is for two separate
purposes:

 - to provide a client certificate and private key, if you use a certificate to authenticate with
   the image registry; and,
 - to provide a CA certificate, if the registry uses a self-signed certificate.

These will often go together, if you are hosting an image registry yourself. All the files in the
secret are expected to be [PEM-encoded][pem-encoding]. This is an ASCII format for certificates and
keys; `openssl` and such tools will typically give you an option of PEM output.

Assuming you have obtained a certificate file and private key and put them in the files `client.crt`
and `client.key` respectively, you can create a secret with `kubectl` like this:

```bash
SECRET_NAME=tls-certs
kubectl create secret generic $SECRET_NAME \
  --from-file=certFile=client.crt \
  --from-file=keyFile=client.key
```

You could also [prepare a secret and encrypt it][sops-guide]; the important bit is that the data
keys in the secret are `certFile` and `keyFile`.

If you have a CA certificate for the client to use, the data key for that is `caFile`. Adapting the
previous example, if you have the certificate in the file `ca.crt`, and the client certificate and
key as before, the whole command would be:

```bash
SECRET_NAME=tls-certs
kubectl create secret generic $SECRET_NAME \
  --from-file=certFile=client.crt \
  --from-file=keyFile=client.key \
  --from-file=caFile=ca.crt
```

### Allow cross-namespace references

To grant access to an `ImageRepository` for policies in other namespaces, the owner of the `ImageRepository`
has to specify a list of label selectors that match the namespaces of the `ImagePolicy` objects.

```yaml
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageRepository
metadata:
  name: app1
  namespace: apps
spec:
  interval: 5m
  image: docker.io/org/image
  secretRef:
    name: regcred
  accessFrom:
    namespaceSelectors:
      - matchLabels:
          kubernetes.io/metadata.name: flux-system
```

**Note** that the `kubernetes.io/metadata.name` is a readonly label added by Kubernetes >= 1.21
automatically on namespaces. If you're using an older version of Kubernetes, please set labels
on the namespaces where the `ImagePolicy` are.

The above definition, allows for `ImagePolicy` in the `flux-system` namespace
to reference the app1 `ImageRepository` e.g.:

```yaml
apiVersion: image.toolkit.fluxcd.io/v1beta1
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

To grant access to all namespaces, an empty `matchLabels` must be provided:

```yaml
  accessFrom:
    namespaceSelectors:
      - matchLabels: {}
```

## Status

```go
// ImageRepositoryStatus defines the observed state of ImageRepository
type ImageRepositoryStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// CannonicalName is the name of the image repository with all the
	// implied bits made explicit; e.g., `docker.io/library/alpine`
	// rather than `alpine`.
	// +optional
	CanonicalImageName string `json:"canonicalImageName,omitempty"`

	// LastScanResult contains the number of fetched tags.
	// +optional
	LastScanResult *ScanResult `json:"lastScanResult,omitempty"`

	meta.ReconcileRequestStatus `json:",inline"`
}
```

The `CanonicalName` field gives the fully expanded image name, filling in any parts left implicit in
the spec. For instance, `alpine` expands to `docker.io/library/alpine`.

The `LastScanResult` field gives a summary of the most recent scan:

```go
type ScanResult struct {
	TagCount int         `json:"tagCount"`
	ScanTime metav1.Time `json:"scanTime,omitempty"`
}
```

### Conditions

There is one condition used: the GitOps toolkit-standard `ReadyCondition`. This will be marked as
true when a scan succeeds, and false when a scan fails.

### Examples

Fetch metadata for a public image every ten minutes:

```yaml
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageRepository
metadata:
  name: podinfo
  namespace: flux-system
spec:
  interval: 10m
  image: ghcr.io/stefanprodan/podinfo
```

Fetch metadata for a private image every minute:

```yaml
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageRepository
metadata:
  name: podinfo
  namespace: flux-system
spec:
  interval: 1m0s
  image: docker.io/org/image
  secretRef:
    name: regcred
```

For private images, you can create a Kubernetes secret in the same namespace
as the `ImageRepository` with `kubectl create secret docker-registry`,
and reference it under `secretRef`.

[image-pull-secrets]: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
[image-auto-provider-secrets]: https://toolkit.fluxcd.io/guides/image-update/#imagerepository-cloud-providers-authentication
[pem-encoding]: https://en.wikipedia.org/wiki/Privacy-Enhanced_Mail
[sops-guide]: https://toolkit.fluxcd.io/guides/mozilla-sops/
[EKS]: https://docs.aws.amazon.com/eks/latest/userguide/what-is-eks.html
[ECR]: https://docs.aws.amazon.com/AmazonECR/latest/userguide/what-is-ecr.html
[GKE]: https://cloud.google.com/kubernetes-engine/docs/concepts/kubernetes-engine-overview
[GCR]: https://cloud.google.com/container-registry/docs/overview
[AKS]: https://docs.microsoft.com/en-us/azure/aks/intro-kubernetes
[ACR]: https://docs.microsoft.com/en-us/azure/container-registry/container-registry-intro
