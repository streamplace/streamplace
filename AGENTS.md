# Streamplace

Streamplace is a Go application. The web and mobile frontend code lives under
`js/`, the media and API packages under `pkg/`, and the command entry points
under `cmd/`. The repo also contains generated lexicon bindings and end-to-end
test infrastructure.

This document is meant to be easy to parse for everyone, but agents should note that they must follow all guidelines, especially the ones marked as such.

Agents: you will want to keep your changes focused on the task. Read the relevant code before you edit it.
Use the repository's existing abstractions and scripts instead of recreating
their behaviour by hand.

## Repository layout

Useful areas of the repository:

- `pkg/`: Go packages, including API, media, RPC, and application logic.
- `cmd/`: Go command entry points.
- `js/app/`: Expo application.
- `js/web/`: web frontend.
- `js/components/`: shared frontend components.
- `js/e2e-web/`: Playwright end-to-end tests.
- `lexicons/`: source lexicons used to generate Go, JS, documentation, and API
  artifacts.
- `hack/`: development, provisioning, and test scripts.
- `.maestro/`: mobile end-to-end flows.

When you work in one of these areas, check its local documentation and
configuration first.

## Development environment

For agents: Inspect the current machine before you make decisions that depend on the
environment.

Use the repository's Make targets and scripts. Do not reproduce their underlying
commands by hand.

For the containerized build, scratch-node, and browser-test workflow, use the
[streamplace-docker skill](.claude/skills/streamplace-docker/SKILL.md). It covers
isolated sibling checkouts and does not require host-native build tools.

## Building

A fresh checkout needs the native build artifacts and the generated frontend
bundles before ordinary Go builds will work.

To set up a fresh checkout, run:

```sh
make dev-setup
```

This sets up Meson and the C and Rust dependencies, and runs an initial frontend
build.

For normal development, run:

```sh
make dev
```

After you change frontend code, rebuild the frontend. Do not assume an existing
`dist` directory reflects the current source:

```sh
make app
```

To build the Go packages that most closely match the CI build, run:

```sh
go build ./pkg/... ./cmd/...
```

For the code you change, run targeted checks where practical:

```sh
go test -count=1 ./pkg/<package>/...
```

`go vet ./pkg/<package>/...` is a fast sanity check, but it is much weaker than
the lint in the commit gate (golangci-lint with staticcheck). A change that
passes `go vet` can still fail CI tests! See [Checking your work](#checking-your-work).

Do not run expensive repository-wide suites when a targeted test gives the same
coverage.

## Frontend

The Go application embeds the built frontend artifacts. A frontend bundle can be
stale after you change source or switch branches.

When you change code under `js/`, make sure the bundle used by builds or
end-to-end tests was generated from the current source.

For UI and component styling, follow the
[streamplace-design skill](.claude/skills/streamplace-design/SKILL.md). Every
visual value comes from a theme token. Do not hardcode raw literals.
Useful checks:

```sh
cd js/app && npx tsc -p . --noEmit
pnpm run check
```

Use the workspace's existing package scripts. Do not add parallel tooling for
formatting, type checking, or dependency analysis.

## Lexicons and generated files

After you change files under `lexicons/`, regenerate the bindings:

```sh
make lexicons
```

Inspect the resulting diff and commit the generated outputs that the change
requires.

Lexicon generation can affect Go code, JS types, documentation, and API
artifacts. Do not assume a generated change is irrelevant just because it is
outside the directory you edited.

If you remove a lexicon, check whether its generated artifacts also need to be
removed.

Do not hand-edit generated output. Change the generator instead.

## Tests

Run the tests relevant to the code you changed.

For Go packages:

```sh
go test -count=1 ./pkg/<package>/...
```

When a package contains slow integration or media tests, use `-run` to run only
the tests you need.

The repository has a local web end-to-end harness and Playwright suite. The
normal entry point is:

```sh
hack/e2e-web-local.sh
```

The harness starts the services the browser tests need and creates temporary test
state. For agents: Use the harness. Do not reproduce its process topology by hand.

When you debug an e2e failure, a good first thing to do is to work out which of these caused it:

- application behaviour
- a stale frontend bundle
- a test harness failure
- an environment or networking failure

For agents: Do not call a failure pre-existing or environment-only until you have verified
it.

## Media and native dependencies

Parts of Streamplace depend on native media libraries and cgo.

If a Go build fails because native libraries or pkg-config metadata are missing,
you will want to inspect the repository's build environment and provisioning scripts.
For agents: Do not install arbitrary host dependencies or invent paths.

Use the repository's configured build environment to compile and test
media-related code.

When you change media code, add targeted tests for the affected package or
pipeline before you run broader suites.

## muxl and other dependencies

Streamplace uses `github.com/streamplace/muxl/go` as a Go dependency.

If a task requires you to test unreleased muxl changes, inspect the current
dependency and the local environment before you add a filesystem `replace`
directive.

Local module replacements are for development only. Do not commit them.

Agents: Do not assume a sibling muxl checkout exists.

## Running Streamplace locally

Use the repository's existing development binaries and scripts.

Agents: Do not assume the ports in the documentation are free on your machine. Check
before you start additional long-running services.

For scratch or test nodes:

- Use isolated data directories.
- Do not inherit unrelated production configuration.
- Do not point development processes at production storage or credentials.
- Use loopback or local-only listeners unless the task requires otherwise.

Do not enable development-only authentication or networking flags in production
configuration.

## Checking your work

A commit must pass more than `go vet` or one package's tests.
To run the full test suite:

```sh
make check   # golangci-lint, pnpm run check, gofmt, cargo check -D warnings
make fix     # autofix: prettier --write, gofmt -w, cargo fix, go mod tidy
```

We use husky for commit hooks. A change that passes `go vet` can still fail hooks!
Run `make check`, or the relevant part of it, before you treat work as done. Do not
bypass the hook with `--no-verify`. When the hook fails, fix the cause.

`pnpm run check` runs `knip`, the ESLint lint, the `js/app` and `e2e-web`
typechecks, and a `prettier --check` over all tracked files. `knip` only looks at
the `js/streamplace` package; see `knip.json` for its narrow reach.

### golangci-lint exclusions

`.golangci.yaml` runs staticcheck with all checks, but it excludes some rules
because generated code trips them. Do not change generated output to satisfy
these rules, and do not re-enable them:

- `ST1003` (underscores in names): indigo codegen trips this.
- `ST1005` (capitalized error strings) and `ST1006` (generic receiver names):
  uniffi-bindgen-go codegen trips these.
- `SA5008` (unknown JSON option): the codegen emits const-based JSON struct tags.
- `QF1003` (could use tagged switch): excluded across the repo as a style nit.
- The `unused` linter is disabled.

### Conventions the linter does not enforce

`make check` runs the full test suite. Reviewers also expect the Go idioms in
[docs/go-conventions.md](docs/go-conventions.md): `log.Log` for the info level
(there is no `log.Info`), `%w` error wrapping, `errors.WriteHTTP*` in HTTP
handlers, and `testify/require` in tests.

## Git and changes

Inspect the repository state before you make changes:

```sh
git status
git diff
```

Agents, please do not:

- Discard unrelated working-tree changes.
- Rewrite commits that are unrelated to the task.
- Commit local filesystem paths.
- Commit temporary module replacements.
- Commit credentials or secrets.
- Assume a particular branch, worktree, stacking, or push workflow unless the
  task or the repository configuration requires it.

Follow the repository's actual branch and contribution state. Do not impose your
own preferred local Git workflow.

Before you finish, inspect the complete diff. Make sure no generated, formatted,
or unrelated changes have leaked into the patch.

## Working style

Understand the existing implementation before you add new abstractions.

Reuse established packages, helpers, scripts, and conventions where they fit.

Some repository documentation records observations that are specific to one
machine. Treat these as context, and verify them against the current checkout and
environment before you rely on them.

For task-specific procedures, use the relevant repository documentation or agent
skills when they exist. Do not apply unrelated operational instructions globally.
