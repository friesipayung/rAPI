# 06 — Release process

The version lives in **one place**: `internal/version/version.go` (`version.Version`).
`build/config.yml` mirrors it for local builds; CI injects the tag version into
`build/config.yml` and `build/linux/nfpm/nfpm.yaml` before packaging.

## Cutting a release

1. Bump `version.Version` in `internal/version/version.go` (e.g. `0.1.0-alpha.2`).
   Keep `build/config.yml` `info.version` in sync (nice for local builds only; CI overrides it).
2. Commit and push to `main`.
3. The **Version tag** workflow (`.github/workflows/version-tag.yml`) runs on every
   push that touches `internal/version/version.go`:
   - reads the version via `go run ./cmd/version`
   - if tag `v<VERSION>` does not exist yet, creates an annotated tag and pushes it
   - if the tag already exists, it exits cleanly (idempotent)
4. The tag push triggers **Build & Release** (`.github/workflows/release.yml`):
   - Linux `.deb`, Linux `.AppImage`, macOS universal `.app` (zipped + sha256)
   - creates the GitHub release; tagged builds are attached to the tag itself

## Pre-release vs stable

`release.yml` marks a tagged build as **pre-release** when the tag has a semver
pre-release suffix (`-alpha.1`, `-rc.1`, …). A tag like `v0.1.0` produces a full release.

## CI builds without a tag

`workflow_dispatch` on `release.yml` builds from `main` and publishes a rolling
pre-release tagged `v0.0.0-ci.<run-number>` (no version file bump needed).

## Local verification

```sh
go run ./cmd/version   # prints the current version
go build -o bin/rapi . # binary logs "rAPI <version>" at startup
```
