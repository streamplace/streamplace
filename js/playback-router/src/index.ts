export interface Env {
  [key: string]: unknown;
}

function getUpstreamServers(env: Env): string[] {
  const servers: string[] = [];
  for (let i = 0; ; i++) {
    const val = env[`UPSTREAM_SERVER__${i}`];
    if (typeof val !== "string" || !val) break;
    servers.push(val);
  }
  return servers;
}

function shuffle<T>(arr: T[]): T[] {
  const result = [...arr];
  for (let i = result.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [result[i], result[j]] = [result[j], result[i]];
  }
  return result;
}

const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, OPTIONS",
  "Access-Control-Allow-Headers": "*",
};

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    if (request.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: corsHeaders });
    }

    const url = new URL(request.url);

    if (
      url.pathname !== "/xrpc/place.stream.playback.getPlaybackServer" ||
      request.method !== "GET"
    ) {
      return new Response("Not Found", { status: 404 });
    }

    const stream = url.searchParams.get("stream");
    if (!stream) {
      return new Response(JSON.stringify({ error: "missing stream param" }), {
        status: 400,
        headers: { "Content-Type": "application/json", ...corsHeaders },
      });
    }

    const upstreamServers = getUpstreamServers(env);
    console.log(
      `Found ${upstreamServers.length} upstream servers:`,
      upstreamServers,
    );

    if (upstreamServers.length === 0) {
      return new Response(
        JSON.stringify({ error: "no upstream servers configured" }),
        {
          status: 500,
          headers: { "Content-Type": "application/json", ...corsHeaders },
        },
      );
    }

    const needed = 3;
    const controller = new AbortController();
    const available: string[] = [];

    const checks = upstreamServers.map(async (server) => {
      const upstreamUrl = `${server}/api/playback/${stream}/hls/index.m3u8`;
      console.log(`Fetching: ${upstreamUrl}`);
      try {
        const res = await fetch(upstreamUrl, {
          signal: controller.signal,
          method: "GET",
        });
        console.log(`${upstreamUrl} -> ${res.status}`);
        if (res.status === 200 && available.length < needed) {
          available.push(server);
          if (available.length >= needed) {
            controller.abort();
          }
        }
      } catch (e) {
        console.log(`${upstreamUrl} -> error: ${e}`);
      }
    });

    await Promise.allSettled(checks);

    if (available.length === 0) {
      return new Response(
        JSON.stringify({ error: "no servers available for this stream" }),
        {
          status: 404,
          headers: { "Content-Type": "application/json", ...corsHeaders },
        },
      );
    }

    return new Response(
      JSON.stringify({ servers: shuffle(available.slice(0, needed)) }),
      {
        status: 200,
        headers: { "Content-Type": "application/json", ...corsHeaders },
      },
    );
  },
};
