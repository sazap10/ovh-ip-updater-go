# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

A single-binary Go service that polls the machine's public IP and updates OVH
Dynamic DNS (DynDNS) records when it changes. Distributed as a multi-arch Docker
image (`sazap10/ovh-ip-updater-go`).

## Commands

```bash
go build                    # build the ovh-ip-updater-go binary
go test ./...               # run all tests
go test -run TestGetIPAddress_Success   # run a single test by name
golangci-lint run           # lint (config in .golangci.yml)
docker compose up -d        # run the service via the prebuilt image
docker compose run lint     # run golangci-lint inside the CI Docker target
```

Toolchain versions are pinned in `.tool-versions` (managed by asdf): Go 1.26.4,
golangci-lint 2.12.2. Note `go.mod` declares `go 1.23` as the minimum language
version — keep it at or below the installed toolchain.

## Configuration

All runtime config comes from environment variables (loaded from a `.env` file
in the working dir if present, via `godotenv`):

- `OVH_USERNAME`, `OVH_PASSWORD` — OVH DynDNS credentials (required)
- `DOMAINS` — comma-separated list of hostnames to update (required)
- `SLEEP_DURATION` — poll interval in seconds (default 3600)
- `BUGSNAG_API_KEY` — optional; enables Bugsnag error reporting

## Architecture

The entire program is `main.go` (~220 lines); `main_test.go` covers it. Key
points for making changes:

- **Main loop** (`main`): fetches the public IP from `api.ipify.org`, and only
  calls the OVH update endpoint for each domain when the IP differs from the
  previous iteration. Sleeps `SLEEP_DURATION` between cycles. Runs forever.
- **Retry wrappers**: `getIPAddressWithRetry` / `setDyndnsIPAddressWithRetry`
  wrap the plain HTTP calls with `cenkalti/backoff/v6` (exponential backoff).
  Both accept variadic `backoff.RetryOption` — this is the seam tests use to
  inject `fastBackoff` (constant 1ms backoff + capped tries) so they don't wait
  on the real schedule.
- **Testability seam**: the IP and OVH URLs are passed as parameters (defaults
  in `defaultIPAddressURL` / `defaultOVHUpdateURL`) so tests point them at
  `httptest` servers. Preserve this pattern — don't hardcode URLs inside the
  request functions.
- **Error reporting** (`notify`): sends to Bugsnag if `BUGSNAG_API_KEY` is set,
  otherwise logs. Errors are wrapped with `pkg/errors`.

When updating the version, change `AppVersion` in the `bugsnag.Configure` call.

## Docker & CI

- `Dockerfile` is multi-stage: `builder` (compiles), `ci` (adds golangci-lint),
  and the final `alpine` image which runs `script/run.sh`. `run.sh` traps
  SIGTERM/SIGINT to forward shutdown to the app and waits ~1s on exit so Bugsnag
  can flush its notifier.
- `.github/workflows/ci.yml` runs on every push: lints via asdf, then builds and
  pushes per-platform images (amd64, arm/v7, arm/v8, arm64) to Docker Hub and
  assembles a multi-arch manifest. Version tags (`vX.Y.Z`) also get `latest`;
  `master` pushes as `latest`. Tagged builds report a release to Bugsnag.

## Conventions

Commits follow Conventional Commits. Dependencies are kept current by Renovate.
