# Release Guide

This document describes the standard release process for hive-operator.

## Overview

The release process follows a branch-based workflow:

- **Main branch** (`main`): The default development branch where new features and bug fixes are merged.
- **Release branch** (`release-x.y`): A long-lived branch for a minor version series
  (e.g., `release-0.4`). Created from `main`, only accepts bug fixes and
  dependency upgrades, no new features. All release tags are created from this
  branch to ensure published code is stable and verified.

## How Versioning Works

The release workflow does **not** require manual version changes in source
files. The Git tag name serves as the single source of truth for the version
number:

1. When a tag (e.g., `0.4.0`) is pushed, the
   [Release workflow](../.github/workflows/release.yml) sets `VERSION` from
   `github.ref_name`
2. **Docker image** — uses `VERSION` directly:
   `quay.io/zncdatadev/hive-operator:<version>`
3. **Helm chart** — `helm package --version $(VERSION) --app-version $(VERSION)`
   overrides the values in `Chart.yaml` during packaging
4. **Helm chart publish** — pushes to
   `quay.io/kubedoopcharts/hive-operator:<version>` (OCI registry)

The `VERSION` in `Makefile` and `version`/`appVersion` in `Chart.yaml` are
development-time defaults (`0.0.0-dev`) on `main`. They do not need to be
updated for a release.

## Release Process

### 1. Prepare Content

Prepare the code to be released before creating the release branch. This
includes merging bug fixes, dependency upgrades, or other stabilization changes
into `main` via Pull Request. Wait for CI to pass and code review before
proceeding.

### 2. Create Release Branch

Create a release branch on the upstream repository from the prepared `main`.

**Via GitHub WebUI:** Navigate to the repository page, click "Branch" →
"New branch", name it `release-0.x` and base it on `main`.

**Via GitHub API / gh CLI:**

```bash
gh api repos/zncdatadev/hive-operator/git/refs \
  -f ref=refs/heads/release-0.x \
  -f sha=$(gh api repos/zncdatadev/hive-operator/git/ref/heads/main --jq .object.sha)
```

Then sync locally:

```bash
git fetch upstream
git checkout release-0.x
```

### 3. Pre-release Verification

Push a `-dev` suffixed tag on the release branch to verify the release
workflow. This confirms the code can be released correctly.

```bash
git pull --rebase upstream release-0.x
git tag x.y.z-dev upstream/release-0.x
git push upstream x.y.z-dev
```

Wait for the release workflow to complete. Verify:

- All jobs pass successfully
- Docker image is available at
  `quay.io/zncdatadev/hive-operator:x.y.z-dev`
- Helm chart is available at
  `quay.io/kubedoopcharts/hive-operator:x.y.z-dev`

If the workflow fails, fix the issue, then delete and re-create the `-dev`
tag:

```bash
git tag -d x.y.z-dev
git push upstream :refs/tags/x.y.z-dev
git tag x.y.z-dev upstream/release-0.x
git push upstream x.y.z-dev
```

Once verified, you can proceed to publish the stable version directly.
There is no need to clean up the `-dev` tag.

### 4. Tag and Publish

```bash
git pull --rebase upstream release-0.x
git tag x.y.z upstream/release-0.x
git push upstream x.y.z
```

This triggers the [Release workflow](../.github/workflows/release.yml) which
runs the following jobs:

- **Markdown Lint** — Lints markdown files under `docs/` and `README.*.md`
- **Golang Lint** — Runs golangci-lint
- **Golang Test** — Runs unit tests
- **Chainsaw Test** — Runs Chainsaw E2E tests across multiple Kubernetes versions and product versions
- **CRD Sync Check** — Verifies CRDs are in sync with manifests
- **Chart Linter (Artifact Hub)** — Validates Helm chart metadata
- **Chart Lint Helm** — Validates the Helm chart with `ct lint` and installs it with `ct install`
- **Chart E2E** — Runs Chainsaw E2E tests against a Helm-installed release
- **Release Image** — Builds and pushes multi-arch Docker image using the root
  Dockerfile to `quay.io/zncdatadev/hive-operator:<version>`, and signs the
  image with Cosign
- **Release Chart** — Publishes the Helm chart to
  `quay.io/kubedoopcharts/hive-operator:<version>` (OCI registry) and
  updates the [kubedoop-helm-charts](https://github.com/zncdatadev/kubedoop-helm-charts)
  index

## Versioning Convention

hive-operator follows [Semantic Versioning](https://semver.org/):

- **Patch** (x.y.Z): Bug fixes, no API changes
- **Minor** (x.Y.z): New features, backward-compatible API changes
- **Major** (X.y.z): Breaking API changes

## Example

Here is an example of releasing version `0.4.0` on the `release-0.4` branch:

```bash
# Step 1: Prepare content on main
# Merge stabilization changes via PR, wait for CI and review

# Step 2: Create release branch on upstream
# Via WebUI or:
gh api repos/zncdatadev/hive-operator/git/refs \
  -f ref=refs/heads/release-0.4 \
  -f sha=$(gh api repos/zncdatadev/hive-operator/git/ref/heads/main --jq .object.sha)

# Sync locally
git fetch upstream
git checkout release-0.4

# Step 3: Pre-release verification
git pull --rebase upstream release-0.4
git tag 0.4.0-dev upstream/release-0.4
git push upstream 0.4.0-dev
# Wait for workflow to pass

# Step 4: Tag and publish (stable version, can only be published once)
git pull --rebase upstream release-0.4
git tag 0.4.0 upstream/release-0.4
git push upstream 0.4.0
```

## Troubleshooting

### Chart release failed

If the `chart-lint-helm` or `release-chart` job fails, check the workflow logs
for details. Common issues include:

- **CRDs out of sync**: Run `make manifests` and `make helm-crd-sync` to
  regenerate CRDs, then commit the changes.
- **Previous tag not found**: For the first release on a new release branch, the
  workflow automatically detects this and marks all charts as changed.

### Re-trigger a pre-release

Only `-dev` pre-release tags can be deleted and re-pushed. To re-trigger:

```bash
git tag -d x.y.z-dev
git push upstream :refs/tags/x.y.z-dev
git tag x.y.z-dev upstream/release-0.x
git push upstream x.y.z-dev
```

**Stable versions (`x.y.z` without suffix) cannot be re-tagged.** If a stable
release fails, you must publish a new patch version (e.g., `x.y.(z+1)`) instead.
