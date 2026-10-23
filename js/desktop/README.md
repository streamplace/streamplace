# Streamplace Desktop

## `pnpm run start`

Boot up Streamplace Desktop for development. Two environment variable options:

- `AQD_SKIP_NODE`: (default `false`): Skip booting up an Streamplace node from
  within the app so you can just test the app against the node you're already
  running locally.

- `AQD_NODE_FRONTEND`: (default `false`): Use frontend bundled with the
  Streamplace node instead of the dev server on `http://localhost:8081`.

## `pnpm run package` / `pnpm run make`

Bundles the node binary (`make <platform>-<arch>`) and the OBS plugin from the
same commit (`make obs-plugin-<platform>-<arch>`, in `bin/`). Desktop installs
or updates that plugin in OBS (`src/obs.ts`); `pnpm run start` uses the copy the
last package run unpacked into `out/obs-plugin/`.

## `pnpm test`

Unit tests for the OBS plugin installer and profile writer.
