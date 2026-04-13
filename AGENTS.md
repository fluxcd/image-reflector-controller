# AGENTS.md

Guidance for AI coding assistants working in `fluxcd/image-reflector-controller`. Read this file before making changes.

## Contribution workflow for AI agents

These rules come from [`fluxcd/flux2/CONTRIBUTING.md`](https://github.com/fluxcd/flux2/blob/main/CONTRIBUTING.md) and apply to every Flux repository.

- **Do not add `Signed-off-by` or `Co-authored-by` trailers with your agent name.** Only a human can legally certify the DCO.
- **Disclose AI assistance** with an `Assisted-by` trailer naming your agent and model:
  ```sh
  git commit -s -m "Add support for X" --trailer "Assisted-by: <agent-name>/<model-id>"
  ```
  The `-s` flag adds the human's `Signed-off-by` from their git config — do not remove it.
- **Commit message format:** Subject in imperative mood ("Add feature X" instead of "Adding feature X"), capitalized, no trailing period, ≤50 characters. Body wrapped at 72 columns, explaining what and why. No `@mentions` or `#123` issue references in the commit — put those in the PR description.
- **Trim verbiage:** in PR descriptions, commit messages, and code comments. No marketing prose, no restating the diff, no emojis.
- **Rebase, don't merge:** Never merge `main` into the feature branch; rebase onto the latest `main` and push with `--force-with-lease`. Squash before merge when asked.
- **Pre-PR gate:** `make tidy fmt vet && make test` must pass and the working tree must be clean after codegen. Commit regenerated files in the same PR.
- **Flux is GA:** Backward compatibility is mandatory. Breaking changes to CRD fields, status, CLI flags, metrics, or observable behavior will be rejected. Design additive changes and keep older API versions round-tripping.
- **Copyright:** All new `.go` files must begin with the boilerplate from `hack/boilerplate.go.txt` (Apache 2.0). Update the year to the current year when copying.
- **Spec docs:** New features and API changes must be documented in `docs/spec/v1/` — `imagepolicies.md` and `imagerepositories.md`. Update the relevant file in the same PR that introduces the change.
- **Tests:** New features, improvements and fixes must have test coverage. Add unit tests in `internal/controller/*_test.go` and other `internal/*` packages as appropriate. Follow the existing patterns for test organization, fixtures, and assertions. Run tests locally before pushing.

## Code quality

Before submitting code, review your changes for the following:

- **No secrets in logs or events.** Never surface registry credentials, cloud provider tokens, or pull-secret contents in error messages, conditions, events, or log lines.
- **No unchecked I/O.** Close HTTP response bodies, file handles, and registry connections in `defer` statements. Check and propagate errors from I/O operations.
- **No unbounded reads.** Use `io.LimitReader` when reading from network sources. Repositories with very large tag sets (tens of thousands) consume significant memory; respect existing limits.
- **No direct Badger imports from reconcilers.** Database access goes through the `DatabaseReader`/`DatabaseWriter` interfaces in `internal/controller/database.go`. That boundary is load-bearing for testability.
- **Registry rate limits.** New scan paths must go through the existing reconcile rate limiter and the `TokenCache`. Do not call `remote.List` in tight loops — public registries (Docker Hub in particular) rate-limit aggressively.
- **Auth through `AuthOptionsGetter`.** Registry auth resolution lives in `internal/registry/options.go`. Add new auth knobs there, not inline in the reconciler. Cloud provider tokens must use the shared `TokenCache`.
- **Error handling.** Wrap errors with `%w` for chain inspection. Do not swallow errors silently. Return actionable error messages that help users diagnose the issue without leaking internal state.
- **Resource cleanup.** Ensure temporary files and directories are cleaned up on all code paths (success and error). Use `defer` and `t.TempDir()` in tests.
- **Concurrency safety.** Do not introduce shared mutable state without synchronization. Reconcilers run concurrently; per-object work must be isolated. Leader election is required because BadgerDB does not support concurrent access from multiple replicas.
- **No panics.** Never use `panic` in runtime code paths. Return errors and let the reconciler handle them gracefully.
- **Minimal surface.** Keep new exported APIs, flags, and environment variables to the minimum needed. The `api/` module is consumed by image-automation-controller — every exported change is a cross-repo contract change.

## Project overview

image-reflector-controller is a component of the [Flux GitOps Toolkit](https://fluxcd.io/flux/components/). It reconciles two CRDs under `image.toolkit.fluxcd.io/v1`:

- `ImageRepository` — scans an OCI image repository at a fixed interval, lists the tags via the registry API, and persists them.
- `ImagePolicy` — references an `ImageRepository`, filters its tag set (optional regex), and elects a "latest" tag using one of three policies: `semver` (Masterminds/semver ranges), `alphabetical`, or `numerical`. It can also reflect the image digest for the elected tag (`digestReflectionPolicy: Never | IfNotPresent | Always`).

Scan results are stored in an on-disk BadgerDB database at `--storage-path` (default `/data`). The elected latest image ref is written to the `ImagePolicy` status and consumed by image-automation-controller, which patches that ref into Git. This controller never writes to Git. Tag listing uses `github.com/google/go-containerregistry` (`remote.List`). Auth resolves via `spec.secretRef`, `spec.serviceAccountName` (with image pull secrets), `spec.certSecretRef`, `spec.proxySecretRef`, and the `fluxcd/pkg/auth` providers for ECR, GCR/AR, and ACR (including object-level workload identity when the feature gate is enabled).

## Repository layout

- `main.go` — controller entrypoint: flag parsing, BadgerDB open, manager setup, reconciler wiring, feature gates, token cache, leader election.
- `api/v1/` — CRD Go types (`imagerepository_types.go`, `imagepolicy_types.go`, `condition_types.go`, `groupversion_info.go`). Its own Go module, imported via `replace` from the root `go.mod`; consumed by image-automation-controller. `api/v1beta1/` and `api/v1beta2/` exist for conversion and must not be broken. `zz_generated.deepcopy.go` is generated.
- `internal/controller/` — `ImageRepositoryReconciler`, `ImagePolicyReconciler`, plus the `DatabaseReader`/`DatabaseWriter` interfaces they depend on. Envtest-based suite in `suite_test.go`.
- `internal/database/` — BadgerDB-backed tag store (`badger.go`) and a periodic GC runnable (`badger_gc.go`) plugged into the manager.
- `internal/policy/` — policy evaluation: `semver.go`, `alphabetical.go`, `numerical.go`, `filter.go` (regex pre-filter with replace), `factory.go` (`PolicerFromSpec`), `policer.go` (the `Policer` interface).
- `internal/registry/` — `AuthOptionsGetter` that turns an `ImageRepository` into go-containerregistry `remote.Option`s (secrets, TLS, proxy, cloud provider auth, token cache). `helper.go` has shared registry helpers.
- `internal/features/` — feature gate registration (workload identity, secret caching).
- `internal/test/` — in-process test registry, TLS server, and proxy used by the unit/envtest suite.
- `config/` — Kustomize manifests: `crd/bases/` (generated CRDs), `manager/`, `rbac/`, `default/`, `samples/`.
- `docs/spec/` — human-written CRD spec docs. `docs/api/v1/` — generated API reference.
- `hack/` — codegen boilerplate and api-docs templates.
- `tests/integration/` — cloud provider e2e tests (AWS/Azure/GCP) using terraform + tftestenv. Separate Go module.

## APIs and CRDs

- Group/version: `image.toolkit.fluxcd.io/v1`. Kinds: `ImageRepository`, `ImagePolicy`. Types in `api/v1/`.
- CRDs under `config/crd/bases/` and API reference docs under `docs/api/v1/image-reflector.md` are generated — do not hand-edit. Regenerate with `make manifests` and `make api-docs`.
- The `api` module is imported by image-automation-controller. Any change to exported API types is a cross-repo contract change. Additive only; breaking changes will be rejected.

## Build, test, lint

All targets in the root `Makefile`. Tool versions pinned via `CONTROLLER_GEN_VERSION` and `GEN_API_REF_DOCS_VERSION`.

- `make tidy` — tidy the root, `api/`, and `tests/integration/` modules.
- `make fmt` / `make vet` — run in root and `api/`.
- `make generate` — `controller-gen object` against `api/` (deepcopy).
- `make manifests` — regenerate CRDs and RBAC into `config/crd/bases` and `config/rbac`.
- `make api-docs` — regenerate `docs/api/v1/image-reflector.md`.
- `make manager` — `go build -o bin/manager main.go`.
- `make test` — runs `tidy generate fmt vet manifests api-docs install-envtest`, then `go test ./... -coverprofile cover.out` at the root and in `api/`. Envtest assets are downloaded via `setup-envtest` and exposed through `KUBEBUILDER_ASSETS`. Honors `GO_TEST_ARGS` for extra flags (e.g. `-run`).
- `make run` — runs the controller against the cluster in `~/.kube/config` using `--storage-path=./data`.
- `make install` / `make deploy` — apply CRDs / full manager via kustomize.
- `make docker-build` — buildx image build. Platforms via `BUILD_PLATFORMS`.

Cloud e2e tests live in `tests/integration/` and have their own `Makefile` (`make test-aws`, `make test-azure`, `make test-gcp`). They require real cloud credentials and provision infrastructure via terraform; do not run them by default.

## Codegen and generated files

Check `go.mod` and the `Makefile` for current dependency and tool versions. After changing API types or kubebuilder markers, regenerate and commit the results:

```sh
make generate manifests api-docs
```

Generated files (never hand-edit):

- `api/v1/zz_generated.deepcopy.go`
- `config/crd/bases/*.yaml`
- `config/rbac/role.yaml`
- `docs/api/v1/image-reflector.md`

No load-bearing `replace` directives beyond the standard `api/` local replace.

Bump `fluxcd/pkg/*` modules as a set. Run `make tidy` after any bump. Use `make <target>` so the pinned tool versions are installed rather than invoking `controller-gen` from elsewhere.

## Conventions

- Standard `gofmt`, `go vet`. All exported names need doc comments; non-trivial unexported types and functions should also have them. Match the style of the file you are editing.
- Reconcilers use `fluxcd/pkg/runtime/patch` + `conditions` for status updates. Do not mutate status in place then `client.Status().Update`; use the patch helper.
- Events go through `fluxcd/pkg/runtime/events.Recorder`; metrics through `fluxcd/pkg/runtime/metrics` via the shared `helper.Metrics`.
- Rate limiting on reconciles is wired via `helper.GetRateLimiter` and the `RateLimiterOptions` flags; do not add your own workqueue rate limiter.
- Honor `spec.provider`, `spec.serviceAccountName`, `spec.secretRef`, `spec.certSecretRef`, `spec.proxySecretRef` when wiring auth options.
- Cloud provider tokens are cached in a `pkg/cache.TokenCache` keyed per involved object; respect that cache in new auth paths.
- Tag filtering: the optional `spec.filterTags` regex runs before the policer; the policer only sees tags that matched and were optionally rewritten via the replace pattern. See `internal/policy/filter.go`.
- Policy selection is dispatched in `internal/policy/factory.go` (`PolicerFromSpec`); add new policy types there and implement the `Policer` interface.
- The `latestTagsCount` constant in `imagerepository_controller.go` controls how many recent tags are surfaced on `ImageRepository` status — do not change it casually.

## Testing

- Unit and envtest suites live next to the code they test (`internal/controller/*_test.go`, `internal/policy/*_test.go`, `internal/database/*_test.go`, `internal/registry/options_test.go`).
- The controller suite (`internal/controller/suite_test.go`) boots a controller-runtime envtest environment; `KUBEBUILDER_ASSETS` must point at an installed kube-apiserver/etcd. `make install-envtest` installs them; `make test` wires the variable for you.
- Registry-touching tests use the in-process helpers in `internal/test/` (plain registry, TLS registry, proxy) backed by `go-containerregistry/pkg/registry`. Prefer these over network calls.
- Fixture images are created in-memory with `go-containerregistry/pkg/v1/random` and pushed to the test registry.
- Run a single test: `make test GO_TEST_ARGS='-run TestImagePolicyReconciler_Reconcile'`.
- `tests/integration/` is a separate module and is **not** exercised by `make test`; do not add unit tests there.

## Gotchas and non-obvious rules

- The Badger database at `--storage-path` is the source of truth for cached tag lists and is assumed to be on a persistent volume in production. Two controller replicas against the same volume is not safe; leader election is enabled for a reason.
- `--storage-value-log-file-size` (default `1<<28`) caps Badger's mmap'd value log. Effective memory is roughly 2× that. Repositories with very large tag sets (tens of thousands) push this. Do not raise the default casually.
- `--gc-interval` (minutes) drives the `BadgerGarbageCollector` runnable registered on the manager. `0` disables GC. Tests should not depend on GC running.
- Public registry rate limits (Docker Hub in particular) are a real source of flakes.
- Semver ranges use Masterminds/semver v3 semantics (`>=1.0.0 <2.0.0`, `~1.2`, `^1.2`). Not Cargo, not npm. Prereleases are excluded unless the range explicitly allows them.
- The `filterTags.extract` replace pattern rewrites the tag string the policer sees; the original tag is still what gets published as `latestRef.tag`. Keep that distinction when adding new policies.
- `DigestReflectionPolicy: Always` is the only mode where `ImagePolicy.spec.interval` is meaningful; the CEL validation rules on the CRD enforce that pairing. Do not loosen them without a very good reason.
- `v1beta1` and `v1beta2` API packages exist for conversion. Removing fields or kinds from those is a breaking change for existing clusters.
- Cloud auth feature gate: `ObjectLevelWorkloadIdentity` changes how `spec.serviceAccountName` is resolved for ECR/GCR/ACR. Changes to auth code paths must consider both gate-on and gate-off behavior, and `auth.InconsistentObjectLevelConfiguration`.
