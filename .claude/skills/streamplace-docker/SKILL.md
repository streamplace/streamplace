---
name: streamplace-docker
description: Build, run, and test Streamplace entirely inside its pinned Docker builder container. Use for cold-start provisioning, Go/cgo and frontend builds, isolated scratch nodes, containerized Playwright checks, or testing local muxl changes across sibling checkouts.
version: 1.0.0
---

# Streamplace Docker development workflow

Keep builds, native dependencies, nodes, and browser tests inside the builder
container. Keep edits and credentialed Git/GitHub operations on the host. Follow
`AGENTS.md` for repository-wide rules; this skill supplies the Docker-specific
procedure, not a branch or PR policy.

## Checkout

Use separate sibling directories such as `streamplace-1` and `streamplace-2`
under any workspace parent. Each checkout needs a unique basename because that
basename determines its container name. Work only in the assigned checkout;
visibility of a sibling does not grant permission to modify it.

## Cold start

```sh
# Host, checkout root; builds and tests are dispatched into the container.
make provision
```

`hack/provision.sh` ensures the container, installs workspace dependencies,
rebuilds missing/stale embedded frontends, runs `make dev`, installs Playwright's
Chromium and system dependencies, and runs `hack/e2e-web-local.sh`. Inside a
container it does that work in place. A successful run proves that checkout's
current build passed the web suite, not that later edits or environment changes
cannot introduce failures. Do not depend on a fixed duration or test count.

To drive the stages separately from the host:

```sh
make dev-container
CONTAINER="$(bash hack/container.sh name)"
docker exec -w "$REPO" "$CONTAINER" make app
docker exec -w "$REPO" "$CONTAINER" make dev
docker exec -w "$REPO" "$CONTAINER" pnpm --filter @streamplace/e2e-web install-browser
docker exec -w "$REPO" "$CONTAINER" hack/e2e-web-local.sh
```

`make dev-container` ensures the container and runs `make dev-setup` if
`build-linux-amd64` is missing. `dev-setup` builds the embedded frontends and
Meson native dependencies; it is not a safe unconditional reconfiguration command.
Inside a container, `make dev-container` invokes `make dev-setup` directly without
the host-side existence guard. Prefer `make dev` for subsequent builds.

Go compilation needs both real frontend bundles (`js/app/dist`, `js/web/dist`)
and the native build. Empty `dist` directories cannot satisfy the Go embed
patterns. Do not install arbitrary host libraries to work around a missing
container build.

## Container management

Run these commands in a **host Bash shell at the checkout root**:

```sh
REPO="$(pwd -P)"
make container
CONTAINER="$(bash hack/container.sh name)"
```

Retain these variables for the host commands below, or reinitialize them in a
new shell. Always pass `-w "$REPO"`; do not rely on a previous shell's directory.

`hack/container.sh`:

- Requires a checkout basename beginning with `streamplace`.
- Reuses a running container named after the checkout or `<checkout>-builder`,
  starts a stopped one, or creates one if neither exists.
- Uses `public.ecr.aws/m4j3c0j7/streamplace:builder-<DOCKERFILE_HASH>`, pinned by
  `.ci/dockerfile-hash.yaml`; it warns about an existing image mismatch but does
  not recreate the container.
- For new containers, mounts the checkout's **entire parent** at the same absolute
  path, making sibling checkouts visible and preserving paths in native metadata.
  This is a broad writable mount: use a trusted builder image and a dedicated
  development workspace, not a parent containing sensitive unrelated data.
- Sets `PKG_CONFIG_PATH` and `LD_LIBRARY_PATH` to this checkout's
  `build-linux-amd64` tree. The current helper assumes Linux amd64; inspect image
  and architecture support before using it on another architecture.

Inspect adopted containers rather than assuming they have those mounts or settings:

```sh
docker ps -a --format '{{.Names}}\t{{.Status}}'
docker inspect "$CONTAINER" --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'
docker exec -w "$REPO" "$CONTAINER" printenv PKG_CONFIG_PATH LD_LIBRARY_PATH
docker exec -w "$REPO" "$CONTAINER" uname -m
```

`docker ps` without `-a` hides stopped containers. Prefer the helper over a second
hand-written `docker run` recipe. Do not replace an existing container or change
its mounts without checking what it is running. Inside a container,
`make container` is a no-op; run subsequent build commands directly there rather
than attempting nested Docker.

## Build and check loop

All commands in this section run from the host using the selected container:

```sh
# After frontend edits, including shared workspace code:
docker exec -w "$REPO" "$CONTAINER" make app
# Re-embed the bundles and rebuild the Go/Rust development binaries:
docker exec -w "$REPO" "$CONTAINER" make dev

# CI-shaped Go build:
docker exec -w "$REPO" "$CONTAINER" env CGO_LDFLAGS=-lm go build ./pkg/... ./cmd/...
```

For affected packages, run `go vet ./pkg/<package>/...` and
`go test -count=1 ./pkg/<package>/...` using the same `docker exec` prefix and
`CGO_LDFLAGS=-lm` environment. Use `-run` for focused media tests instead of
assuming a whole media suite will be quick. Build-time cgo resolution needs
`PKG_CONFIG_PATH`; running linked binaries needs `LD_LIBRARY_PATH`. Inspect both
when adopting a container built by another workflow.

Other checks, when relevant:

```sh
docker exec -w "$REPO" "$CONTAINER" go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run -c ./.golangci.yaml
docker exec -w "$REPO" "$CONTAINER" gofmt -l pkg cmd
docker exec -w "$REPO/js/app" "$CONTAINER" pnpm exec tsc -p . --noEmit
docker exec -w "$REPO" "$CONTAINER" pnpm run check
# Required after lexicon edits; review generated changes on the host.
docker exec -w "$REPO" "$CONTAINER" make lexicons
```

### Branch changes and stale artifacts

- `make dev` and `make app-cached` skip frontend builds when both bundle entrypoints
  exist. Run `make app` explicitly after frontend changes, then `make dev` before
  testing embedded UI. Provisioning's tracked-file timestamp heuristic does not
  replace this rule, especially for new untracked source files.
- After changing branches or dependency manifests, run `pnpm install` inside the
  container. Regenerate lexicons when their source or generator changes; generated
  JS types on disk can otherwise describe a different branch.
- The native build directory is not branch-aware. `make dev` only checks whether
  it exists. If Meson configuration or native dependencies changed, inspect the
  appropriate reconfiguration/rebuild path; get permission before deleting an
  existing build tree. An existing directory alone does not prove setup finished.
- Generated outputs for deleted lexicons may need explicit removal. Review the
  complete generated diff; do not commit unrelated formatter changes.

Prefer a checkout's own native build. If deliberately reusing a sibling build,
verify compatible source, architecture, toolchain, and libraries, and ensure its
original absolute path is visible in the container: `.pc` files contain absolute
paths. Set both native-library environment variables to that build for the
specific command. Do not assume a sibling exists or modify its artifacts.

## Isolated scratch node

`make dev` builds binaries; it does **not** start a server. The generated
`build-linux-amd64/streamplace` launcher assigns a frontend proxy to port 38081,
so an environment override cannot disable it. For a scratch node serving the
embedded bundle, invoke `libstreamplace` directly.

Enter the container from the host:

```sh
docker exec -it -w "$REPO" "$CONTAINER" bash
```

Then, **inside the container at the checkout root**, inspect listeners with
`ss -ltnup`. The following ports are examples, not reservations: select unused
ports in this container's network namespace for every enabled service. Separate
containers may reuse internal ports; processes sharing a network namespace may
not. Keep internal/admin listeners private.

```sh
SCRATCH="$(mktemp -d "${TMPDIR:-/tmp}/streamplace-scratch.XXXXXX")"
env -i PATH="$PATH" HOME="$HOME" \
  LD_LIBRARY_PATH="$PWD/build-linux-amd64/lib" \
  SP_DATA_DIR="$SCRATCH" SP_NO_FIREHOSE=true \
  SP_HTTP_ADDR=127.0.0.1:38090 SP_HTTP_INTERNAL_ADDR=127.0.0.1:39091 \
  SP_HTTPS_ADDR=127.0.0.1:38444 \
  SP_RTMP_ADDR=127.0.0.1:19350 SP_RTMPS_ADDR=127.0.0.1:19351 \
  SP_RTMPS_ADDON_ADDR=127.0.0.1:19352 \
  SP_MIST_ADMIN_PORT=14243 SP_MIST_RTMP_PORT=11936 SP_MIST_HTTP_PORT=28081 \
  SP_DEV_FRONTEND_PROXY=false SP_DEV_PUBLIC_OAUTH=true \
  SP_BROADCASTER_HOST=localhost:38090 \
  ./build-linux-amd64/libstreamplace
```

The clean environment avoids inheriting production storage, access policies,
credentials, and relay configuration. Unsetting the relay host alone restores
its default; `SP_NO_FIREHOSE=true` disables firehose consumption. Confirm startup
logs select local file storage, not S3, and do not report an unexpected frontend
proxy. `SP_DEV_PUBLIC_OAUTH=true` permits local HTTP OAuth development; never use
it in production. Loading client metadata is not proof of a completed login.

For live frontend development instead, set the scratch node's proxy to
`http://127.0.0.1:38091` and run this in a second container shell at `js/app`:

```sh
EXPO_PUBLIC_STREAMPLACE_URL=http://127.0.0.1:38090 pnpm exec expo start --port 38091
```

Keep backend URL and selected ports consistent. The explicit URL overrides the
app's development environment default. Do not set `CI=1` when file watching is
needed. For automated sessions, use the harness's process supervisor rather than
leaving an unmanaged background server. Stop only processes belonging to this run
and remove only its temporary data when finished.

## Browser and end-to-end verification

The helper creates no published host ports. Container loopback is not host
loopback: run the browser in the same container, or deliberately arrange a private
forward to the service. Do not recreate a working build container just to add a
port mapping. Bind any browser debugging endpoint to loopback and keep it private.

Resolve Chromium through the package that owns Playwright; never hardcode a
revision or try to execute a container cache path on the host:

```sh
# Host shell:
docker exec -w "$REPO/js/e2e-web" "$CONTAINER" \
  node -p 'require("playwright").chromium.executablePath()'
docker exec -w "$REPO" "$CONTAINER" hack/e2e-web-local.sh
```

For direct Chromium automation, headless mode and `--disable-dev-shm-usage` can be
useful in containers. Use `--no-sandbox` only when necessary in a trusted
isolated development container, not as a general browser policy.

The web harness creates local PDS/PLC services and a temporary account, starts a
server and looping WHIP stream, and passes `SERVER_URL`, `ACCOUNT_HANDLE`, and
`ACCOUNT_DID` to Playwright. Its global setup waits for the live stream. Diagnose
harness readiness and ingest failures separately from UI failures. Local private
service access may require `SP_TRUST_PRIVATE_NETWORK=true`; keep that allowance
scoped to the development harness, never production configuration.

Let `hack/e2e-web-local.sh` manage cleanup. Do not run concurrent copies against
the same checkout: its cleanup tracks newly appearing processes for that binary.
If interrupted outside normal cleanup, inspect ownership before stopping orphaned
server, dev-env, or ingest processes; avoid broad `pkill` patterns that could kill
another checkout's work. Defunct processes hold no ports; distinguish them from
live orphans. Use the workspace's required Node version in the builder rather
than borrowing an arbitrary host Node installation.

For native iOS simulator work, use the separate `streamplace-app` skill; that is
not part of this Docker-only workflow.

## Testing unreleased muxl

Only when the task requires local muxl changes:

1. Locate the intended muxl checkout and confirm it is mounted at the same path
   inside the builder. Do not assume a sibling checkout exists.
2. In that checkout's configured build environment, run `just build-go-wasm` to
   rebuild `go/muxl.wasm`; the Go module embeds that blob, so stale Wasm tests old
   behavior.
3. Temporarily replace `github.com/streamplace/muxl/go` in this checkout's `go.mod`
   with the actual mounted muxl `go` directory. Do not overwrite an existing
   replacement without understanding it.
4. Run the relevant Streamplace tests inside the container.
5. Remove only the temporary replacement you introduced. Never commit a local
   filesystem replacement; normal adoption requires a published version and a
   dependency update.

## Host-side Git boundary

Run status, diff review, commits, pushes, and GitHub operations on the host using
its existing identity and credentials. The bind mount shares files, not SSH trust
or authentication. Do not copy credentials into the builder or invent a second
Git identity. Follow the repository's actual branch and contribution workflow;
container provisioning does not select a base branch or require stacked PRs.
