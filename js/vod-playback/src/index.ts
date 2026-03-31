/**
 * VOD Playback Worker
 *
 * Generic AT Protocol data proxy. Resolves AT-URIs to content stored in S3.
 * Currently supports place.stream.video playback via HLS.
 *
 * S3 bucket layout:
 *   {did}/{collection}/{rkey}/metadata.json
 *   blobs/{cid}.mp4
 */

interface Env {
  S3_BUCKET_URL: string;
}

interface TrackSegment {
  offset: number;
  size: number;
  durationTicks: number;
  sampleCount: number;
}

interface TrackMeta {
  type: "video" | "audio";
  codec: string;
  timescale: number;
  initCid: string;
  segments: TrackSegment[];
  width?: number;
  height?: number;
  channels?: number;
  sampleRate?: number;
}

interface VideoMeta {
  blobCid: string;
  blobSize: number;
  tracks: Record<string, TrackMeta>;
}

const CORS_HEADERS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, HEAD, OPTIONS",
  "Access-Control-Allow-Headers": "Range",
  "Access-Control-Expose-Headers": "Content-Length, Content-Range",
};

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    if (request.method === "OPTIONS") {
      return new Response(null, { headers: CORS_HEADERS });
    }

    const url = new URL(request.url);
    const path = url.pathname;

    try {
      if (path === "/xrpc/place.stream.playback.getVideoPlaylist") {
        return await handleGetVideoPlaylist(url, env);
      }
      if (path === "/xrpc/place.stream.playback.getInitSegment") {
        return await handleGetInitSegment(url, env);
      }
      if (path === "/xrpc/place.stream.playback.getVideoBlob") {
        return await handleGetVideoBlob(url, request, env);
      }
      return new Response("Not Found", { status: 404 });
    } catch (e: any) {
      if (e instanceof XRPCError) {
        return jsonResponse({ error: e.name, message: e.message }, e.status);
      }
      return new Response(e.message || "Internal Server Error", {
        status: 500,
      });
    }
  },
};

// ---------------------------------------------------------------------------
// AT-URI parsing
// ---------------------------------------------------------------------------

interface ParsedURI {
  did: string;
  collection: string;
  rkey: string;
}

function parseATURI(uri: string): ParsedURI {
  // at://did:plc:abc123/place.stream.video/3mi2ikg6gij26
  const match = uri.match(/^at:\/\/(did:[^/]+)\/([^/]+)\/([^/]+)$/);
  if (!match) {
    throw new XRPCError(400, "InvalidRequest", `Invalid AT-URI: ${uri}`);
  }
  return { did: match[1], collection: match[2], rkey: match[3] };
}

function requireURI(url: URL): ParsedURI {
  const uri = url.searchParams.get("uri");
  if (!uri) {
    throw new XRPCError(400, "InvalidRequest", "uri parameter is required");
  }
  const parsed = parseATURI(uri);
  if (parsed.collection !== "place.stream.video") {
    throw new XRPCError(
      400,
      "InvalidRequest",
      `Unsupported collection: ${parsed.collection}`,
    );
  }
  return parsed;
}

// ---------------------------------------------------------------------------
// S3 path helpers
// ---------------------------------------------------------------------------

function didToDir(did: string): string {
  return did.replace(/:/g, "-");
}

function s3MetaPath(parsed: ParsedURI): string {
  return `${didToDir(parsed.did)}/${parsed.collection}/${parsed.rkey}/metadata.json`;
}

// ---------------------------------------------------------------------------
// Errors & helpers
// ---------------------------------------------------------------------------

class XRPCError extends Error {
  constructor(
    public status: number,
    public override name: string,
    message: string,
  ) {
    super(message);
  }
}

function jsonResponse(data: any, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json", ...CORS_HEADERS },
  });
}

// ---------------------------------------------------------------------------
// Metadata cache
// ---------------------------------------------------------------------------

const metaCache = new Map<string, { meta: VideoMeta; expires: number }>();

async function fetchMeta(parsed: ParsedURI, env: Env): Promise<VideoMeta> {
  const key = `${parsed.did}/${parsed.rkey}`;
  const cached = metaCache.get(key);
  if (cached && cached.expires > Date.now()) {
    return cached.meta;
  }

  const metaUrl = `${env.S3_BUCKET_URL}/${s3MetaPath(parsed)}`;
  console.log(`[fetchMeta] ${metaUrl}`);
  const resp = await fetch(metaUrl);
  if (!resp.ok) {
    console.log(`[fetchMeta] ${resp.status} ${resp.statusText}`);
    throw new XRPCError(404, "VideoNotFound", `No video found for ${key}`);
  }
  const meta: VideoMeta = await resp.json();
  metaCache.set(key, { meta, expires: Date.now() + 60_000 });
  return meta;
}

// ---------------------------------------------------------------------------
// place.stream.playback.getVideoPlaylist
// ---------------------------------------------------------------------------

async function handleGetVideoPlaylist(url: URL, env: Env): Promise<Response> {
  const parsed = requireURI(url);
  const track = url.searchParams.get("track");
  const meta = await fetchMeta(parsed, env);

  const playlist = track
    ? mediaPlaylist(meta, track, parsed)
    : masterPlaylist(meta, parsed);

  return new Response(playlist, {
    headers: {
      "Content-Type": "application/vnd.apple.mpegurl",
      ...CORS_HEADERS,
    },
  });
}

function atURI(parsed: ParsedURI): string {
  return `at://${parsed.did}/${parsed.collection}/${parsed.rkey}`;
}

function masterPlaylist(meta: VideoMeta, parsed: ParsedURI): string {
  const uri = atURI(parsed);
  const lines: string[] = ["#EXTM3U", "#EXT-X-VERSION:6", ""];

  for (const [tid, t] of Object.entries(meta.tracks)) {
    if (t.type === "audio") {
      const trackUri = xrpcURL("place.stream.playback.getVideoPlaylist", {
        uri,
        track: tid,
      });
      lines.push(
        `#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="audio${tid}",` +
          `DEFAULT=YES,AUTOSELECT=YES,CHANNELS="${t.channels ?? 2}",URI="${trackUri}"`,
      );
    }
  }
  lines.push("");

  const audioCodec = Object.values(meta.tracks).find(
    (t) => t.type === "audio",
  )?.codec;

  for (const [tid, t] of Object.entries(meta.tracks)) {
    if (t.type === "video") {
      const totalBytes = t.segments.reduce((s, seg) => s + seg.size, 0);
      const totalTicks = t.segments.reduce(
        (s, seg) => s + seg.durationTicks,
        0,
      );
      const totalSamples = t.segments.reduce(
        (s, seg) => s + seg.sampleCount,
        0,
      );
      const totalDur = totalTicks / t.timescale;
      const bandwidth =
        totalDur > 0 ? Math.round((totalBytes * 8) / totalDur) : 0;
      const frameRate = totalDur > 0 ? totalSamples / totalDur : 0;
      const codecs = audioCodec ? `${t.codec},${audioCodec}` : t.codec;

      const trackUri = xrpcURL("place.stream.playback.getVideoPlaylist", {
        uri,
        track: tid,
      });
      lines.push(
        `#EXT-X-STREAM-INF:AUDIO="audio",BANDWIDTH=${bandwidth},` +
          `CODECS="${codecs}",RESOLUTION=${t.width}x${t.height},` +
          `FRAME-RATE=${frameRate.toFixed(3)}`,
      );
      lines.push(trackUri);
    }
  }

  return lines.join("\n") + "\n";
}

function mediaPlaylist(
  meta: VideoMeta,
  trackId: string,
  parsed: ParsedURI,
): string {
  const t = meta.tracks[trackId];
  if (!t) {
    throw new XRPCError(404, "TrackNotFound", `Track ${trackId} not found`);
  }

  const uri = atURI(parsed);

  const maxDurSec = t.segments.reduce(
    (m, seg) => Math.max(m, seg.durationTicks / t.timescale),
    0,
  );
  const targetDuration = Math.max(1, Math.ceil(maxDurSec));

  const initURI = xrpcURL("place.stream.playback.getInitSegment", {
    uri,
    track: trackId,
  });

  const blobURI =
    xrpcURL("place.stream.playback.getVideoBlob", {
      uri,
      cid: meta.blobCid,
    }) + ".mp4";

  const lines: string[] = [
    "#EXTM3U",
    "#EXT-X-VERSION:6",
    "#EXT-X-PLAYLIST-TYPE:VOD",
    "#EXT-X-INDEPENDENT-SEGMENTS",
    `#EXT-X-TARGETDURATION:${targetDuration}`,
    "#EXT-X-MEDIA-SEQUENCE:0",
    `#EXT-X-MAP:URI="${initURI}"`,
    "",
  ];

  for (const seg of t.segments) {
    const durSec = seg.durationTicks / t.timescale;
    lines.push(`#EXTINF:${durSec.toFixed(6)},`);
    lines.push(`#EXT-X-BYTERANGE:${seg.size}@${seg.offset}`);
    lines.push(blobURI);
  }

  lines.push("#EXT-X-ENDLIST");
  return lines.join("\n") + "\n";
}

// ---------------------------------------------------------------------------
// place.stream.playback.getInitSegment
// ---------------------------------------------------------------------------

async function handleGetInitSegment(url: URL, env: Env): Promise<Response> {
  const parsed = requireURI(url);
  const track = url.searchParams.get("track");
  if (!track) {
    throw new XRPCError(400, "InvalidRequest", "track parameter is required");
  }

  const meta = await fetchMeta(parsed, env);
  const t = meta.tracks[track];
  if (!t) {
    throw new XRPCError(404, "TrackNotFound", `Track ${track} not found`);
  }

  const initUrl = `${env.S3_BUCKET_URL}/blobs/${t.initCid}.mp4`;
  console.log(`[getInitSegment] ${initUrl}`);
  const resp = await fetch(initUrl);
  if (!resp.ok) {
    throw new XRPCError(404, "TrackNotFound", "Init segment not available");
  }

  return new Response(resp.body, {
    headers: {
      "Content-Type": "video/mp4",
      "Cache-Control": "public, max-age=31536000, immutable",
      ...CORS_HEADERS,
    },
  });
}

// ---------------------------------------------------------------------------
// place.stream.playback.getVideoBlob
// ---------------------------------------------------------------------------

async function handleGetVideoBlob(
  url: URL,
  request: Request,
  env: Env,
): Promise<Response> {
  const parsed = requireURI(url);

  let cid = url.searchParams.get("cid") ?? "";
  // Strip file extension appended for player compatibility
  cid = cid.replace(/\.(mp4|m4s)$/, "");

  const headers: Record<string, string> = {};
  const range = request.headers.get("Range");
  if (range) {
    headers["Range"] = range;
  }

  const blobUrl = `${env.S3_BUCKET_URL}/blobs/${cid}.mp4`;
  console.log(`[getVideoBlob] ${blobUrl}`, headers);
  const resp = await fetch(blobUrl, { headers });

  if (!resp.ok && resp.status !== 206) {
    throw new XRPCError(404, "BlobNotFound", "Blob not available");
  }

  const responseHeaders: Record<string, string> = {
    "Content-Type": "video/mp4",
    "Cache-Control": "public, max-age=31536000, immutable",
    ...CORS_HEADERS,
  };

  const contentRange = resp.headers.get("Content-Range");
  if (contentRange) {
    responseHeaders["Content-Range"] = contentRange;
  }
  const contentLength = resp.headers.get("Content-Length");
  if (contentLength) {
    responseHeaders["Content-Length"] = contentLength;
  }

  return new Response(resp.body, {
    status: resp.status,
    headers: responseHeaders,
  });
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function xrpcURL(method: string, params: Record<string, string>): string {
  const qs = Object.entries(params)
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
    .join("&");
  return `${method}?${qs}`;
}
