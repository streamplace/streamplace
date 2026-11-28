# shellcheck shell=bash
# Paths whose contents end up in the embedded frontend bundles. Provisioning
# uses them to decide whether a stale bundle exists; warm seeding uses them to
# force a rebuild when they differ from the bundle's source commit.
# shellcheck disable=SC2034 # used by the scripts that source this file
FRONTEND_INPUTS=(
  js/app/src
  js/app/components
  js/app/store
  js/app/public
  js/app/scripts
  js/app/app.config.ts
  js/app/package.json
  js/web
  js/components/src
  js/streamplace/src
  js/streamplace/package.json
  js/i18n/locales
  js/i18n/i18next.config.js
  js/brand/generate.mjs
  package.json
  pnpm-lock.yaml
  pnpm-workspace.yaml
)
