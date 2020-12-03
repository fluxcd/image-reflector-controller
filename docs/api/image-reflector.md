<h1>Image reflector API reference</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#image.toolkit.fluxcd.io%2fv1alpha1">image.toolkit.fluxcd.io/v1alpha1</a>
</li>
</ul>
<h2 id="image.toolkit.fluxcd.io/v1alpha1">image.toolkit.fluxcd.io/v1alpha1</h2>
<p>Package v1alpha1 contains API types for the image v1alpha1 API
group. These types are concerned with reflecting metadata from OCI
image repositories into a cluster, so they can be consulted for
e.g., automation.</p>
Resource Types:
<ul class="simple"></ul>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ImagePolicy">ImagePolicy
</h3>
<p>ImagePolicy is the Schema for the imagepolicies API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicySpec">
ImagePolicySpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>imageRepositoryRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImageRepositoryRef points at the object specifying the image
being scanned</p>
</td>
</tr>
<tr>
<td>
<code>policy</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicyChoice">
ImagePolicyChoice
</a>
</em>
</td>
<td>
<p>Policy gives the particulars of the policy to be followed in
selecting the most recent image</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicyStatus">
ImagePolicyStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ImagePolicyChoice">ImagePolicyChoice
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicySpec">ImagePolicySpec</a>)
</p>
<p>ImagePolicyChoice is a union of all the types of policy that can be
supplied.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>semver</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.SemVerPolicy">
SemVerPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SemVer gives a semantic version range to check against the tags
available.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ImagePolicySpec">ImagePolicySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicy">ImagePolicy</a>)
</p>
<p>ImagePolicySpec defines the parameters for calculating the
ImagePolicy</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>imageRepositoryRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImageRepositoryRef points at the object specifying the image
being scanned</p>
</td>
</tr>
<tr>
<td>
<code>policy</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicyChoice">
ImagePolicyChoice
</a>
</em>
</td>
<td>
<p>Policy gives the particulars of the policy to be followed in
selecting the most recent image</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ImagePolicyStatus">ImagePolicyStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicy">ImagePolicy</a>)
</p>
<p>ImagePolicyStatus defines the observed state of ImagePolicy</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>latestImage</code><br>
<em>
string
</em>
</td>
<td>
<p>LatestImage gives the first in the list of images scanned by
the image repository, when filtered and ordered according to
the policy.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ImageRepository">ImageRepository
</h3>
<p>ImageRepository is the Schema for the imagerepositories API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImageRepositorySpec">
ImageRepositorySpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>image</code><br>
<em>
string
</em>
</td>
<td>
<p>Image is the name of the image repository</p>
</td>
</tr>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Interval is the length of time to wait between
scans of the image repository.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout for image scanning.
Defaults to &lsquo;Interval&rsquo; duration.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef can be given the name of a secret containing
credentials to use for the image registry. The secret should be
created with <code>kubectl create secret docker-registry</code>, or the
equivalent.</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent image scans.
It does not apply to already started scans. Defaults to false.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImageRepositoryStatus">
ImageRepositoryStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ImageRepositorySpec">ImageRepositorySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImageRepository">ImageRepository</a>)
</p>
<p>ImageRepositorySpec defines the parameters for scanning an image
repository, e.g., <code>fluxcd/flux</code>.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>image</code><br>
<em>
string
</em>
</td>
<td>
<p>Image is the name of the image repository</p>
</td>
</tr>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>Interval is the length of time to wait between
scans of the image repository.</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Timeout for image scanning.
Defaults to &lsquo;Interval&rsquo; duration.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>SecretRef can be given the name of a secret containing
credentials to use for the image registry. The secret should be
created with <code>kubectl create secret docker-registry</code>, or the
equivalent.</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>This flag tells the controller to suspend subsequent image scans.
It does not apply to already started scans. Defaults to false.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ImageRepositoryStatus">ImageRepositoryStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImageRepository">ImageRepository</a>)
</p>
<p>ImageRepositoryStatus defines the observed state of ImageRepository</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
<tr>
<td>
<code>canonicalImageName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>CannonicalName is the name of the image repository with all the
implied bits made explicit; e.g., <code>docker.io/library/alpine</code>
rather than <code>alpine</code>.</p>
</td>
</tr>
<tr>
<td>
<code>lastScanResult</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ScanResult">
ScanResult
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastScanResult contains the number of fetched tags.</p>
</td>
</tr>
<tr>
<td>
<code>ReconcileRequestStatus</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#ReconcileRequestStatus">
github.com/fluxcd/pkg/apis/meta.ReconcileRequestStatus
</a>
</em>
</td>
<td>
<p>
(Members of <code>ReconcileRequestStatus</code> are embedded into this type.)
</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.ScanResult">ScanResult
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImageRepositoryStatus">ImageRepositoryStatus</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tagCount</code><br>
<em>
int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scanTime</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1alpha1.SemVerPolicy">SemVerPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1alpha1.ImagePolicyChoice">ImagePolicyChoice</a>)
</p>
<p>SemVerPolicy specifices a semantic version policy.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>range</code><br>
<em>
string
</em>
</td>
<td>
<p>Range gives a semver range for the image tag; the highest
version within the range that&rsquo;s a tag yields the latest image.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
