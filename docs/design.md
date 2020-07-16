# Design of image update automation

There are three parts to the design, that operate independently, and
in sympathy with one another:

 - the job runner (type UpdateJob)
 - the image metadata reflector (types ImageRepository, ImagePolicy)
 - the automation controller (type ImageAutomation)

Some tooling comes alongside these parts:

 - the `tk-image` command-line tool lets you create the resource
   mentioned above, and can be used as an extension to the gitops
   toolkit command-line tool.
 - the `image-update` image can be used with `kpt fn`, GitHub Actions,
   and `UpdateJob` to update images in resources within a working
   directory.

## Update job controller

The job controller runs `UpdateJob` resources, each of which specifies
an update operation on a git repository. Each run checks out the given
repo at the specified ref, runs the specified image with the
arguments, then commits and pushes as specified.

### Integration

The automation controller and the command-line tool `tk-image` create
`UpdateJob` resources for making changes within a git repository.

The jobs created by the automation controller and the command-line
tool use `image-update` as the image to run on the git repo.

### Design notes

This could be a more specific `UpdateImageJob`, but it would differ
only in the update being done, so it is a small step from there to a
general job.

The motivation for making the jobs separate to the automation is that
you can then do ad-hoc updates by creating an `UpdateJob` from
command-line tooling. The downside is that it needs another moving
part, albeit a generally-useful one.

## Image metadata reflector

The image metadata reflector reconciles `ImageRepository` and
`ImagePolicy` resources. An `ImageRepository` specifies an image
repository to scan, and an `ImagePolicy` selects a specific image from
a repository according to given rules. The purpose of these is to make
that information available to some other system within the cluster.

### Integration

The automation controller creates these as indicated by information
from its specification and the repository it looks at; and it consults
these resources when constructing `UpdateJob` resources to run.

### Design notes

The image repository and policy are separated so that different
policies can be derived from the same image repository specification.
Credentials only need to be specified once, for the repository object,
rather than maintained for all the policies.

An alternative is to _just_ have policies, and infer the repositories
that need to be scanned. This would mean less to do e.g., for the
automation (it could just directly create each policy as it finds
it). It would make the implementation of the controller more
complicated though, since it would need to maintain internal state for
the repository scanning, rather than being able to consult
`ImageRepository` resources.

## Automation controller

The automation controller monitors `ImageAutomation` resources, which
specify a git repository on which to run automation and a
specification of how to update the repository. For each of these, it:

 - calculates which image repositories need to be scanned, and the
   policies for updating them, according to the specification given in
   the `ImageAutomation` and git repository;
 - consults the policies it created to determine updates to perform;
 - creates and manages `UpdateJob` resources to run the updates.

### Specification

To be designed -- see the notes below.

### Design notes

There's a large design space for the automation, along various axes:

 - where does the specification for what is automated live -- in git
   or in a resource?
 - is the specification part of the resource/file it automates, or
   separate?
 - does the specification name all the things to which it applies; or
   does it work with rules or patterns?

So, for example, one design could lie at this point in the space: an
`ImageAutomation` resource names a git repository, a workload object,
and an `ImagePolicy`; every time the policy selects an image that does
not match what is given for the workload resource in git, a job is
created to update it.

It is easy to see how to implement this design, since everything is
totally explicit -- you just do what the resources say. But it's not
great for the user, because they have to spend time spelling it all
out for the controller, and do the work of keeping the automation and
policy objects in the cluster up to date with what's in the git repo
(one way to do that is to keep them in the git repo and let them be
synced; but in general, the workloads are not going to run in the same
place as the automation, so it would be fiddly to keep these in the
same place).

A design in the other direction would be to expect annotations,
similar to those used by Flux v1, on workloads to be automated. The
controller would interpret those to determine which image repositories
and policies are needed.

This might be tricky when the automation is managed by a different
team to that in charge of the application configuration -- in that
scenario, the annotations would have to be carefully applied after the
fact (perhaps with a kustomization), which may as well mean the
annotations are kept in a separate file.

## Easy (Flux v1) mode

This design includes an "easy mode" for simple deployments, which
mimics closely the behaviour of the automation in Flux v1. Creating an
`ImageAutomation` resource that analyses the annotations in git
repository will give much the same results as in Flux v1.

## Extension points

**Actions other than UpdateJob**

As a way in to other systems, e.g., Tekton, the `ImageAutomation` job
could specify an action other than creating an `UpdateJob` (for
Tekton, this might be creating a `TaskRun`). The impedence here is how
to package the information about what to update -- this will need to
be thought through for each such integration.

**Supply a different image to UpdateJob**

If the way this design makes commits doesn't suit, then a different
image can be supplied to the `UpdateJob` resource created by
automation.

**Writing a controller**

As a last resort, it's possible to write a controller that reacts to
`ImagePolicy` changes in its own way, or analyses the files in the git
repository according to some other scheme. It can hook back into the
rest of the design by using `UpdateJob` resources to accomplish
whatever it calculates needs to be done.

## Open questions

**How does the controller authenticate with image repositories?**

Flux v1 picks up credentials from the cluster, e.g., by looking at the
image pull secrets for a workload while it's looking for images to
scan. It also tries to sense when it can get credentials from the
environment, e.g., by asking GCP for credentials. That basically
works, modulo getting some details right and keeping up with platform
changes.

In this design, it would be possible to adapt the Flux v1 scheme, more
or less. For an `ImageRepository`, the controller could look through
all the workloads to find one using the image, and use its image pull
secrets. And, it could try to sense when it can use ambient
authorisation.

It might be better to at least start by being explicit, though, and
specify image pull secrets or the use of ambient authorisation in the
`ImageRepository`. This way it is easier to troubleshoot.

**How will image metadata access be authorised?**

The image metadata controller makes image metadata available to anyone
who can create and read `ImageRepository` and `ImagePolicy`
resources. This is not an obvious attack route, but anything which
expands the API surface, especially if it uses credentials (image pull
secrets) but doesn't require authorisation itself, is a potential
vulnerability.

**Cluster-scoped resources**

Flux v1 will look at everything you give it access to. In this design,
automation is controlled by resources, which are namespaced. To make
the "automate everything you see" behaviour accessible, it would be
necessary to have a cluster-scope resource.
