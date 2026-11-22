// Cloudflare entry point: the only file that knows about the platform.

import { createEdge, type EdgeHandler } from "./handler";

interface Env {
  ASSETS: { fetch(request: Request): Promise<Response> };
  PROXY_SECRET?: string;
  [key: string]: unknown;
}

function upstreams(env: Env): string[] {
  const servers: string[] = [];
  for (let i = 0; ; i++) {
    const value = env[`UPSTREAM_SERVER__${i}`];
    if (typeof value !== "string" || !value) break;
    servers.push(value);
  }
  return servers;
}

// Env is fixed for the isolate's deployment, so the template and the
// down-marking survive between requests.
let edge: EdgeHandler | undefined;

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    edge ??= createEdge({
      upstreams: upstreams(env),
      proxySecret: env.PROXY_SECRET,
      assets: (req) => env.ASSETS.fetch(req),
    });
    return edge(request, request.headers.get("CF-Connecting-IP"));
  },
};
