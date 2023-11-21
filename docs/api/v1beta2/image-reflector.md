<h1>Image reflector API reference v1beta2</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#image.toolkit.fluxcd.io%2fv1beta2">image.toolkit.fluxcd.io/v1beta2</a>
</li>
</ul>
<h2 id="image.toolkit.fluxcd.io/v1beta2">image.toolkit.fluxcd.io/v1beta2</h2>
<p>Package v1beta2 contains API types for the image API group, version
v1beta2. These types are concerned with reflecting metadata from
OCI image repositories into a cluster, so they can be consulted for
e.g., automation.</p>
Resource Types:
<ul class="simple"></ul>
<h3 id="image.toolkit.fluxcd.io/v1beta2.AlphabeticalPolicy">AlphabeticalPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicyChoice">ImagePolicyChoice</a>)
</p>
<p>AlphabeticalPolicy specifies a alphabetical ordering policy.</p>
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
<code>order</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Order specifies the sorting order of the tags. Given the letters of the
alphabet as tags, ascending order would select Z, and descending order
would select A.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1beta2.ImagePolicy">ImagePolicy
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
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicySpec">
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
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#NamespacedObjectReference">
github.com/fluxcd/pkg/apis/meta.NamespacedObjectReference
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
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicyChoice">
ImagePolicyChoice
</a>
</em>
</td>
<td>
<p>Policy gives the particulars of the policy to be followed in
selecting the most recent image</p>
</td>
</tr>
<tr>
<td>
<code>filterTags</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1beta2.TagFilter">
TagFilter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FilterTags enables filtering for only a subset of tags based on a set of
rules. If no rules are provided, all the tags from the repository will be
ordered and compared.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicyStatus">
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
<h3 id="image.toolkit.fluxcd.io/v1beta2.ImagePolicyChoice">ImagePolicyChoice
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicySpec">ImagePolicySpec</a>)
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
<a href="#image.toolkit.fluxcd.io/v1beta2.SemVerPolicy">
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
<tr>
<td>
<code>alphabetical</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1beta2.AlphabeticalPolicy">
AlphabeticalPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Alphabetical set of rules to use for alphabetical ordering of the tags.</p>
</td>
</tr>
<tr>
<td>
<code>numerical</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1beta2.NumericalPolicy">
NumericalPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Numerical set of rules to use for numerical ordering of the tags.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1beta2.ImagePolicySpec">ImagePolicySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicy">ImagePolicy</a>)
</p>
<p>ImagePolicySpec defines the parameters for calculating the
ImagePolicy.</p>
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
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#NamespacedObjectReference">
github.com/fluxcd/pkg/apis/meta.NamespacedObjectReference
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
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicyChoice">
ImagePolicyChoice
</a>
</em>
</td>
<td>
<p>Policy gives the particulars of the policy to be followed in
selecting the most recent image</p>
</td>
</tr>
<tr>
<td>
<code>filterTags</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1beta2.TagFilter">
TagFilter
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FilterTags enables filtering for only a subset of tags based on a set of
rules. If no rules are provided, all the tags from the repository will be
ordered and compared.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1beta2.ImagePolicyStatus">ImagePolicyStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicy">ImagePolicy</a>)
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
<tr>
<td>
<code>observedPreviousImage</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedPreviousImage is the observed previous LatestImage. It is used
to keep track of the previous and current images.</p>
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
</td>
</tr>
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
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1beta2.ImageRepository">ImageRepository
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
<a href="#image.toolkit.fluxcd.io/v1beta2.ImageRepositorySpec">
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
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretRef can be given the name of a secret containing
credentials to use for the image registry. The secret should be
created with <code>kubectl create secret docker-registry</code>, or the
equivalent.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountName is the name of the Kubernetes ServiceAccount used to authenticate
the image pull if the service account has attached pull secrets.</p>
</td>
</tr>
<tr>
<td>
<code>certSecretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertSecretRef can be given the name of a Secret containing
either or both of</p>
<ul>
<li>a PEM-encoded client certificate (<code>tls.crt</code>) and private
key (<code>tls.key</code>);</li>
<li>a PEM-encoded CA certificate (<code>ca.crt</code>)</li>
</ul>
<p>and whichever are supplied, will be used for connecting to the
registry. The client cert and key are useful if you are
authenticating with a certificate; the CA cert is useful if
you are using a self-signed server certificate. The Secret must
be of type <code>Opaque</code> or <code>kubernetes.io/tls</code>.</p>
<p>Note: Support for the <code>caFile</code>, <code>certFile</code> and <code>keyFile</code> keys has
been deprecated.</p>
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
<tr>
<td>
<code>accessFrom</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/acl#AccessFrom">
github.com/fluxcd/pkg/apis/acl.AccessFrom
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessFrom defines an ACL for allowing cross-namespace references
to the ImageRepository object based on the caller&rsquo;s namespace labels.</p>
</td>
</tr>
<tr>
<td>
<code>exclusionList</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExclusionList is a list of regex strings used to exclude certain tags
from being stored in the database.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The provider used for authentication, can be &lsquo;aws&rsquo;, &lsquo;azure&rsquo;, &lsquo;gcp&rsquo; or &lsquo;generic&rsquo;.
When not specified, defaults to &lsquo;generic&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Insecure, if set to true indicates that the image registry is hosted at an
HTTP endpoint.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImageRepositoryStatus">
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
<h3 id="image.toolkit.fluxcd.io/v1beta2.ImageRepositorySpec">ImageRepositorySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImageRepository">ImageRepository</a>)
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
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretRef can be given the name of a secret containing
credentials to use for the image registry. The secret should be
created with <code>kubectl create secret docker-registry</code>, or the
equivalent.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountName is the name of the Kubernetes ServiceAccount used to authenticate
the image pull if the service account has attached pull secrets.</p>
</td>
</tr>
<tr>
<td>
<code>certSecretRef</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CertSecretRef can be given the name of a Secret containing
either or both of</p>
<ul>
<li>a PEM-encoded client certificate (<code>tls.crt</code>) and private
key (<code>tls.key</code>);</li>
<li>a PEM-encoded CA certificate (<code>ca.crt</code>)</li>
</ul>
<p>and whichever are supplied, will be used for connecting to the
registry. The client cert and key are useful if you are
authenticating with a certificate; the CA cert is useful if
you are using a self-signed server certificate. The Secret must
be of type <code>Opaque</code> or <code>kubernetes.io/tls</code>.</p>
<p>Note: Support for the <code>caFile</code>, <code>certFile</code> and <code>keyFile</code> keys has
been deprecated.</p>
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
<tr>
<td>
<code>accessFrom</code><br>
<em>
<a href="https://godoc.org/github.com/fluxcd/pkg/apis/acl#AccessFrom">
github.com/fluxcd/pkg/apis/acl.AccessFrom
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessFrom defines an ACL for allowing cross-namespace references
to the ImageRepository object based on the caller&rsquo;s namespace labels.</p>
</td>
</tr>
<tr>
<td>
<code>exclusionList</code><br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExclusionList is a list of regex strings used to exclude certain tags
from being stored in the database.</p>
</td>
</tr>
<tr>
<td>
<code>provider</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The provider used for authentication, can be &lsquo;aws&rsquo;, &lsquo;azure&rsquo;, &lsquo;gcp&rsquo; or &lsquo;generic&rsquo;.
When not specified, defaults to &lsquo;generic&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>insecure</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Insecure, if set to true indicates that the image registry is hosted at an
HTTP endpoint.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1beta2.ImageRepositoryStatus">ImageRepositoryStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImageRepository">ImageRepository</a>)
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
<p>CanonicalName is the name of the image repository with all the
implied bits made explicit; e.g., <code>docker.io/library/alpine</code>
rather than <code>alpine</code>.</p>
</td>
</tr>
<tr>
<td>
<code>lastScanResult</code><br>
<em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ScanResult">
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
<code>observedExclusionList</code><br>
<em>
[]string
</em>
</td>
<td>
<p>ObservedExclusionList is a list of observed exclusion list. It reflects
the exclusion rules used for the observed scan result in
spec.lastScanResult.</p>
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
<h3 id="image.toolkit.fluxcd.io/v1beta2.NumericalPolicy">NumericalPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicyChoice">ImagePolicyChoice</a>)
</p>
<p>NumericalPolicy specifies a numerical ordering policy.</p>
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
<code>order</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Order specifies the sorting order of the tags. Given the integer values
from 0 to 9 as tags, ascending order would select 9, and descending order
would select 0.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1beta2.ScanResult">ScanResult
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImageRepositoryStatus">ImageRepositoryStatus</a>)
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
<tr>
<td>
<code>latestTags</code><br>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="image.toolkit.fluxcd.io/v1beta2.SemVerPolicy">SemVerPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicyChoice">ImagePolicyChoice</a>)
</p>
<p>SemVerPolicy specifies a semantic version policy.</p>
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
<h3 id="image.toolkit.fluxcd.io/v1beta2.TagFilter">TagFilter
</h3>
<p>
(<em>Appears on:</em>
<a href="#image.toolkit.fluxcd.io/v1beta2.ImagePolicySpec">ImagePolicySpec</a>)
</p>
<p>TagFilter enables filtering tags based on a set of defined rules</p>
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
<code>pattern</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Pattern specifies a regular expression pattern used to filter for image
tags.</p>
</td>
</tr>
<tr>
<td>
<code>extract</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Extract allows a capture group to be extracted from the specified regular
expression pattern, useful before tag evaluation.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
