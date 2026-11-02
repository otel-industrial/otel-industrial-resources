# Releasing

The Go modules in this repo are versioned in lockstep, the same way as
[opentelemetry-collector](https://github.com/open-telemetry/opentelemetry-collector)
and [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib).
Every module in the `industrial` set in [`versions.yaml`](../versions.yaml) is
tagged with the same version on every release, whether or not it changed.
The repo-level tag (`v0.2.0`) then builds the distributions in
`otel-industrial-collector/manifests`, whose in-repo receivers carry that same
version.

The tooling is [multimod](https://github.com/open-telemetry/opentelemetry-go-build-tools/tree/main/multimod),
pinned in `internal/tools/go.mod` and run through the root `Makefile`.

## Adding a module

Add its module path to the `industrial` set in `versions.yaml`. Modules in
this repo require each other at the set version and keep a `replace` to the
local path. Every receiver, processor, exporter, extension, or connector
module must be in at least one manifest, pinned at the set version, and every
manifest's `dist.version` is the set version. CI enforces all of this with
`multimod verify` and `.github/scripts/check-release-coverage.sh`, and
`excluded-modules` is limited to the modules that script allows.

The manifests pin the in-repo modules by tag, with no `replaces:`, so they
build on their own outside this repo. CI builds them with `make LOCAL=1`, which
adds replaces for every in-repo module, so a PR builds against the code in the
checkout.

## Preparing a release

Run these in a regular clone. multimod cannot read the git state of a linked
worktree.

1. Bump the OTel collector core and contrib modules to the upstream release
   you are building against, and commit the result:

   ```sh
   make update-otel OTEL_VERSION=0.164.0 OTEL_STABLE_VERSION=1.70.0 OTEL_CONTRIB_VERSION=0.164.1
   ```

   This updates the in-repo `go.mod` files, the manifests and builder
   configs, the OCB version, and the docs, then runs
   `.github/scripts/check-otel-versions.sh`. Move the receivers from outside
   OTel to their latest commit with `make update-opcua` and
   `make update-sparkplug`, or pin one with `VERSION=<tag, branch, or commit>`.
   `prepare-release` only changes this repo's version.
2. Create a branch and run, with the current version from `versions.yaml` as
   `PREVIOUS_VERSION`:

   ```sh
   make prepare-release PREVIOUS_VERSION=0.1.0 RELEASE_CANDIDATE=0.2.0
   ```

   `versions.yaml` starts at `0.0.1`, so the first release is
   `PREVIOUS_VERSION=0.0.1 RELEASE_CANDIDATE=0.1.0`.

   This updates `versions.yaml`, the `dist.version` and receiver versions in
   the manifests, and every intra-repo `require` in the `go.mod` files, and
   commits the result.
3. Open a PR and merge it once CI passes.

## Tagging

From an up-to-date `main` at the merged release commit:

```sh
make push-tags COMMIT=<release commit>
git tag -a v0.2.0 <release commit> -m v0.2.0
git push <remote> v0.2.0
```

`REMOTE` defaults to the SSH URL of this repo; over HTTPS, pass
`REMOTE=https://github.com/otel-industrial/otel-industrial-resources.git`, or
`REMOTE=origin` if `origin` is this repo rather than a fork. Push the repo tag
to the same remote.

`push-tags` tags every module in the set (for example
`otel-industrial-collector/receiver/modbusreceiver/v0.2.0`) and pushes them in
one atomic push. If `COMMIT` is not checked out, it offers to check it out and
returns you to your branch afterwards; without a terminal it stops instead. It
stops before tagging if any of the tags already exist on the remote. If the
push fails or is interrupted, it deletes the tags it created, so you can fix
the problem and run it again. Push the module tags before the repo tag. The repo tag starts the `release` workflow, which builds the
distributions from the module tags, as anyone using the manifests would, and
creates a draft GitHub release. Review the draft and publish it.
