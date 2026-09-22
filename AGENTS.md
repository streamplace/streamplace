# Streamplace

Streamplace is a Go application with web/mobile frontend code under `js/`, media
and API packages under `pkg/`, command entry points under `cmd/`, generated
lexicon bindings, and end-to-end test infrastructure.

Keep changes focused on the requested task. Read the relevant code before
editing, and prefer the repository's existing abstractions and scripts over
recreating their behaviour manually.

## Repository layout

Useful areas of the repository include:

- `pkg/` — Go packages, including API, media, RPC, and application logic.
- `cmd/` — Go command entry points.
- `js/app/` — Expo application.
- `js/web/` — web frontend.
- `js/components/` — shared frontend components.
- `js/e2e-web/` — Playwright end-to-end tests.
- `lexicons/` — source lexicons used to generate Go, JS, documentation, and API
  artifacts.
- `hack/` — development, provisioning, and test scripts.
- `.maestro/` — mobile end-to-end flows.

Check package-local documentation and configuration when working in one of these
areas.

## Development environment

Inspect the current machine before making environment-dependent decisions.

Prefer repository-provided Make targets and scripts over manually reproducing
their underlying commands.

## Building

A fresh checkout needs both the native build artifacts and the generated
frontend bundles before ordinary Go builds will work correctly.

The repository provides a cold-start entry point; ask first before executing
this as it does a lot:

```sh
make provision
```

Use it when the current environment supports the repository's provisioning
workflow.

For normal development:

```sh
make dev
```

After changing frontend code, rebuild the frontend rather than assuming an
existing `dist` directory reflects the current source:

```sh
make app
```

The Go packages most closely corresponding to the CI build are:

```sh
go build ./pkg/... ./cmd/...
```

For code you change, prefer targeted checks where practical:

```sh
go vet ./pkg/<package>/...
go test -count=1 ./pkg/<package>/...
```

Avoid running very expensive repository-wide suites without a reason when a
targeted test provides equivalent coverage.

## Frontend

The Go application embeds built frontend artifacts. An existing frontend bundle
may be stale after source changes or branch changes.

When changing code under `js/`, make sure the bundle used by builds or
end-to-end tests was generated from the current source.

Useful checks include:

```sh
cd js/app && npx tsc -p . --noEmit
pnpm run check
```

Use the workspace's existing package scripts rather than introducing parallel
tooling for formatting, type checking, or dependency analysis.

## Lexicons and generated files

Changes under `lexicons/` require regeneration:

```sh
make lexicons
```

Inspect the resulting diff and commit the required generated outputs.

Lexicon generation can affect Go code, JS types, documentation, and API
artifacts. Do not assume generated changes are irrelevant merely because they
are outside the directory you directly edited.

If a lexicon is removed, check whether generated artifacts associated with it
also need to be removed.

Do not hand-edit generated output when the corresponding generator should be
changed instead.

## Tests

Run tests relevant to the code being modified.

For Go packages:

```sh
go test -count=1 ./pkg/<package>/...
```

Use `-run` for focused testing when a package contains slow integration or media
tests.

The repository contains a local web end-to-end harness and Playwright suite. The
normal entry point is:

```sh
hack/e2e-web-local.sh
```

The harness starts the services required by the browser tests and creates
temporary test state. Prefer the provided harness over manually reproducing its
process topology.

When debugging an e2e failure, distinguish between:

- application behaviour
- frontend bundle staleness
- test harness failure
- environment/networking failure

Do not classify a failure as pre-existing or environment-only without verifying
it.

## Media and native dependencies

Parts of Streamplace depend on native media libraries and cgo.

If a Go build fails because native libraries or pkg-config metadata are missing,
inspect the repository's build environment and provisioning scripts rather than
installing arbitrary host dependencies or inventing paths.

Prefer the repository's configured build environment for media-related
compilation and tests.

Changes to media code should generally receive targeted tests for the affected
package or pipeline before broader suites are attempted.

## muxl

Streamplace consumes `github.com/streamplace/muxl/go` as a Go dependency.

If a task specifically requires testing unreleased muxl changes, inspect the
current dependency and local environment before introducing a filesystem
`replace` directive.

A local module replacement is development-only and must not be committed.

Do not assume a sibling muxl checkout exists.

## Running Streamplace locally

Use the repository's existing development binaries and scripts.

Do not assume that ports mentioned in documentation are available on the current
machine. Check before starting additional long-running services.

For scratch or test nodes:

- use isolated data directories;
- avoid inheriting unrelated production configuration;
- do not point development processes at production storage or credentials;
- prefer loopback/local-only listeners unless the task requires otherwise.

Do not enable development-only authentication or networking flags in production
configuration.

## Git and changes

Inspect repository state before making changes:

```sh
git status
git diff
```

Do not:

- discard unrelated working-tree changes;
- rewrite commits that are unrelated to the task;
- commit local filesystem paths;
- commit temporary module replacements;
- commit credentials or secrets;
- assume a particular branch, worktree, stacking, or push workflow unless the
  task or repository configuration requires it.

Follow the repository's actual branch and contribution state rather than
imposing a preferred local Git workflow.

Before finishing, inspect the complete diff and make sure generated, formatted,
or unrelated changes have not leaked into the patch.

## Working style

Prefer understanding the existing implementation before adding new abstractions.

Reuse established packages, helpers, scripts, and conventions where they fit.

When repository documentation contains machine-specific observations, treat them
as context rather than universal requirements. Verify them against the current
checkout and environment.

For task-specific procedures, use relevant repository documentation or agent
skills when available instead of applying unrelated operational instructions
globally.
