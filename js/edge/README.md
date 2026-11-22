# @streamplace/edge

A small stateless worker in front of a Streamplace broadcaster's apex
hostname. It serves the web app shell from the edge, so the site stays up
through node restarts, and sends everything that needs data to a healthy node.

It is strictly progressive enhancement: a single node with no worker has every
capability, and nothing in the node requires one.

## Routes

| Request                                                                                                                                                          | Action                                  |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `GET`/`HEAD` on `/xrpc/place.stream.playback.{getLivePlaylist,getLiveSegment,getVideoPlaylist,getVideoBlob}`                                                     | 307 to the same path on a healthy node  |
| Non-`GET`/`HEAD`; `/xrpc/`, `/api/`, `/oauth/`, `/.well-known/`, `/dl/`; `/app-updates`, `/linkbanner.png`, `/favicon.ico`, `/favicon.png`, `/notification-icon` | Reverse-proxy to a healthy node         |
| An existing static asset                                                                                                                                         | Served with the node's cache policy     |
| A missing `_expo/static/` file or asset-like path                                                                                                                | 404                                     |
| Anything else                                                                                                                                                    | App shell with the page head spliced in |

The head comes from the node's `GET /api/page-head?url=<page>`, which returns
the same HTML the node would put between `<!-- sp-head -->` and
`<!-- /sp-head -->` in its own template. When no node answers, the worker
serves the template untouched: the default tags inside the markers keep the
page presentable.

Playback is redirected rather than proxied because playlists carry
root-relative segment URLs: after the first redirect every poll and segment
stays on that node, and media never bills per request at the edge.

## Configuration

- `UPSTREAM_SERVER__0`, `UPSTREAM_SERVER__1`, … — node origins, in `wrangler.toml`.
- `PROXY_SECRET` — a secret (`wrangler secret put PROXY_SECRET`). It must match
  the node's `--proxy-secret`; the worker sends it in `X-Proxy-Secret` along
  with the real client IP in `X-Forwarded-For`.
- Assets: `node scripts/copy-assets.mjs` copies `js/app/dist` into `assets/`,
  skipping source maps and the native OTA bundles (nodes serve those).

## Node flags

Behind the worker, a node needs:

- `--server-host <its own hostname>`, different from `--broadcaster-host` (the
  worker's hostname). This keeps `did:web:<broadcaster>` resolving to the
  broadcaster document and points video uploads at the node itself.
- `--proxy-secret <same secret>`, so per-IP limits use the forwarded client
  rather than the worker's shared egress address.

## Develop and deploy

```sh
node scripts/copy-assets.mjs
pnpm exec wrangler dev --var UPSTREAM_SERVER__0:http://127.0.0.1:38090
pnpm test
pnpm exec wrangler deploy
```

`hack/e2e-web-local.sh --edge` runs the Playwright suite through the worker.
