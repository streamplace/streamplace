/**
 * VOD Playback Worker
 *
 * Resolves AT Protocol place.stream.video records to HLS playlists.
 * Fetches the record from the AppView to find the source blob CID,
 * then looks up blob-level metadata from S3.
 *
 * S3 bucket layout:
 *   blobs/{cid}.mp4          — archive / sidecar data
 *   blobs/{cid}.json         — per-blob playback metadata (tracks, segments)
 */

interface Env {
  S3_BUCKET_URL: string;
  BLOB_CDN_URL?: string;
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
  blobCid?: string;
  blobSize?: number;
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
    console.error(`[parseATURI] Invalid AT-URI: ${uri}`);
    throw new XRPCError(400, "InvalidRequest", `Invalid AT-URI: ${uri}`);
  }
  return { did: match[1], collection: match[2], rkey: match[3] };
}

function requireURI(url: URL): ParsedURI {
  const uri = url.searchParams.get("uri");
  if (!uri) {
    console.error(`[requireURI] uri parameter is required`);
    throw new XRPCError(400, "InvalidRequest", "uri parameter is required");
  }
  const parsed = parseATURI(uri);
  if (parsed.collection !== "place.stream.video") {
    console.error(`[requireURI] Unsupported collection: ${parsed.collection}`);
    throw new XRPCError(
      400,
      "InvalidRequest",
      `Unsupported collection: ${parsed.collection}`,
    );
  }
  return parsed;
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
// Record resolution — fetch the place.stream.video record from the AppView
// ---------------------------------------------------------------------------

interface VideoRecord {
  source:
    | {
        $type: "place.stream.muxl.defs#archive";
        data: { ref: { $link: string }; size: number };
      }
    | {
        $type: "place.stream.muxl.defs#archiveBlob";
        ref: string;
        size: number;
        start?: number;
        end?: number;
      };
}

interface ResolvedVideo {
  meta: VideoMeta;
  /** Clip bounds from the record's source (nanoseconds), if present. */
  recordStartNs?: number;
  recordEndNs?: number;
}

// ---------------------------------------------------------------------------
// DID resolution — resolve DID to PDS service endpoint
// ---------------------------------------------------------------------------

interface DIDDocument {
  service?: { id: string; type: string; serviceEndpoint: string }[];
}

const pdsCache = new Map<string, { pdsUrl: string; expires: number }>();

async function resolvePDS(did: string): Promise<string> {
  const cached = pdsCache.get(did);
  if (cached && cached.expires > Date.now()) {
    return cached.pdsUrl;
  }

  let didDocUrl: string;
  if (did.startsWith("did:plc:")) {
    didDocUrl = `https://plc.directory/${did}`;
  } else if (did.startsWith("did:web:")) {
    const domain = did.slice("did:web:".length);
    didDocUrl = `https://${domain}/.well-known/did.json`;
  } else {
    throw new XRPCError(
      400,
      "InvalidRequest",
      `Unsupported DID method: ${did}`,
    );
  }

  console.log(`[resolvePDS] ${didDocUrl}`);
  const resp = await fetch(didDocUrl);
  if (!resp.ok) {
    throw new XRPCError(404, "VideoNotFound", `Could not resolve DID: ${did}`);
  }

  const doc: DIDDocument = await resp.json();
  const pds = doc.service?.find((s) => s.id === "#atproto_pds");
  if (!pds) {
    throw new XRPCError(
      404,
      "VideoNotFound",
      `No PDS service found for ${did}`,
    );
  }

  pdsCache.set(did, {
    pdsUrl: pds.serviceEndpoint,
    expires: Date.now() + 300_000,
  });
  return pds.serviceEndpoint;
}

// ---------------------------------------------------------------------------
// Record fetching — resolve DID → PDS → getRecord
// ---------------------------------------------------------------------------

const recordCache = new Map<string, { record: VideoRecord; expires: number }>();

async function fetchRecord(parsed: ParsedURI): Promise<VideoRecord> {
  const key = `${parsed.did}/${parsed.rkey}`;
  const cached = recordCache.get(key);
  if (cached && cached.expires > Date.now()) {
    return cached.record;
  }

  const pdsUrl = await resolvePDS(parsed.did);
  const url =
    `${pdsUrl}/xrpc/com.atproto.repo.getRecord` +
    `?repo=${encodeURIComponent(parsed.did)}` +
    `&collection=${encodeURIComponent(parsed.collection)}` +
    `&rkey=${encodeURIComponent(parsed.rkey)}`;
  console.log(`[fetchRecord] ${url}`);

  const resp = await fetch(url);
  if (!resp.ok) {
    console.log(`[fetchRecord] ${resp.status} ${resp.statusText}`);
    throw new XRPCError(404, "VideoNotFound", `Record not found: ${key}`);
  }
  const body: any = await resp.json();
  const record = body.value as VideoRecord;

  recordCache.set(key, { record, expires: Date.now() + 60_000 });
  return record;
}

function blobCidFromRecord(record: VideoRecord): string {
  const src = record.source;
  if (src.$type === "place.stream.muxl.defs#archive") {
    return src.data.ref.$link;
  }
  if (src.$type === "place.stream.muxl.defs#archiveBlob") {
    return src.ref;
  }
  throw new XRPCError(
    400,
    "InvalidRecord",
    "Unsupported source type on record",
  );
}

// ---------------------------------------------------------------------------
// Blob metadata — keyed by blob CID in S3
// ---------------------------------------------------------------------------

const metaCache = new Map<string, { meta: VideoMeta; expires: number }>();

async function fetchBlobMeta(blobCid: string, env: Env): Promise<VideoMeta> {
  const cached = metaCache.get(blobCid);
  if (cached && cached.expires > Date.now()) {
    return cached.meta;
  }

  const metaUrl = `${env.S3_BUCKET_URL}/blobs/${blobCid}.json`;
  console.log(`[fetchBlobMeta] ${metaUrl}`);
  const resp = await fetch(metaUrl);
  if (!resp.ok) {
    console.log(`[fetchBlobMeta] ${resp.status} ${resp.statusText}`);
    throw new XRPCError(
      404,
      "VideoNotFound",
      `No blob metadata for ${blobCid}`,
    );
  }
  const meta: VideoMeta = await resp.json();
  metaCache.set(blobCid, { meta, expires: Date.now() + 60_000 });
  return meta;
}

/**
 * Resolve a video AT-URI to its blob metadata and any record-level clip bounds.
 */
async function resolveVideo(
  parsed: ParsedURI,
  env: Env,
): Promise<ResolvedVideo> {
  const record = await fetchRecord(parsed);
  const blobCid = blobCidFromRecord(record);
  const meta = await fetchBlobMeta(blobCid, env);

  const src = record.source;
  const recordStartNs =
    src.$type === "place.stream.muxl.defs#archiveBlob" ? src.start : undefined;
  const recordEndNs =
    src.$type === "place.stream.muxl.defs#archiveBlob" ? src.end : undefined;

  return { meta, recordStartNs, recordEndNs };
}

// ---------------------------------------------------------------------------
// place.stream.playback.getVideoPlaylist
// ---------------------------------------------------------------------------

async function handleGetVideoPlaylist(url: URL, env: Env): Promise<Response> {
  const parsed = requireURI(url);
  const track = url.searchParams.get("track");
  const { meta, recordStartNs, recordEndNs } = await resolveVideo(parsed, env);

  // Query-param start/end (nanoseconds) override or further narrow the record's clip bounds.
  let startNs = url.searchParams.has("start")
    ? parseInt(url.searchParams.get("start")!, 10)
    : recordStartNs;
  let endNs = url.searchParams.has("end")
    ? parseInt(url.searchParams.get("end")!, 10)
    : recordEndNs;

  if (startNs !== undefined && isNaN(startNs)) {
    throw new XRPCError(400, "InvalidRequest", "start must be an integer");
  }
  if (endNs !== undefined && isNaN(endNs)) {
    throw new XRPCError(400, "InvalidRequest", "end must be an integer");
  }
  if (startNs !== undefined && endNs !== undefined && startNs >= endNs) {
    throw new XRPCError(400, "InvalidRequest", "start must be less than end");
  }

  const playlist = track
    ? mediaPlaylist(meta, track, parsed, env, startNs, endNs)
    : masterPlaylist(meta, parsed, startNs, endNs);

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

function masterPlaylist(
  meta: VideoMeta,
  parsed: ParsedURI,
  startNs?: number,
  endNs?: number,
): string {
  const uri = atURI(parsed);
  const lines: string[] = ["#EXTM3U", "#EXT-X-VERSION:6", ""];

  // Build optional time-range params to forward to media playlists
  const timeParams: Record<string, string> = {};
  if (startNs !== undefined) timeParams.start = String(startNs);
  if (endNs !== undefined) timeParams.end = String(endNs);

  // Prefer AAC as default audio for Safari compatibility
  const audioEntries = Object.entries(meta.tracks).filter(
    ([, t]) => t.type === "audio",
  );
  const defaultTid =
    audioEntries.find(([, t]) => t.codec.startsWith("mp4a"))?.[0] ??
    audioEntries[0]?.[0];

  for (const [tid, t] of audioEntries) {
    const trackUri = xrpcURL("place.stream.playback.getVideoPlaylist", {
      uri,
      track: tid,
      ...timeParams,
    });
    const isDefault = tid === defaultTid;
    if (!isDefault) {
      continue;
    }
    lines.push(
      `#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="${t.codec}",` +
        `DEFAULT=${isDefault ? "YES" : "NO"},AUTOSELECT=YES,` +
        `CHANNELS="${t.channels ?? 2}",URI="${trackUri}"`,
    );
  }
  lines.push("");

  // Prefer AAC for CODECS string (Safari compatibility)
  const audioTracks = Object.values(meta.tracks).filter(
    (t) => t.type === "audio",
  );
  const audioCodec =
    audioTracks.find((t) => t.codec.startsWith("mp4a"))?.codec ??
    audioTracks[0]?.codec ??
    "mp4a.40.2";

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
        ...timeParams,
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
  env: Env,
  startNs?: number,
  endNs?: number,
): string {
  const t = meta.tracks[trackId];
  if (!t) {
    console.error(`[mediaPlaylist] Track ${trackId} not found`);
    throw new XRPCError(404, "TrackNotFound", `Track ${trackId} not found`);
  }

  const uri = atURI(parsed);

  // Filter segments to the requested time range. Each segment that overlaps
  // [startNs, endNs) is included in full (we can't split mid-GOP).
  const segments = filterSegments(t.segments, t.timescale, startNs, endNs);

  const maxDurSec = segments.reduce(
    (m, seg) => Math.max(m, seg.durationTicks / t.timescale),
    0,
  );
  const targetDuration = Math.max(1, Math.ceil(maxDurSec));

  const initURI = xrpcURL("place.stream.playback.getInitSegment", {
    uri,
    track: trackId,
  });

  const trackBlobCid = t.blobCid ?? meta.blobCid;
  const blobURI = env.BLOB_CDN_URL
    ? `${env.BLOB_CDN_URL}/blobs/${trackBlobCid}.mp4`
    : xrpcURL("place.stream.playback.getVideoBlob", {
        uri,
        cid: trackBlobCid,
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

  for (const seg of segments) {
    const durSec = seg.durationTicks / t.timescale;
    lines.push(`#EXTINF:${durSec.toFixed(6)},`);
    lines.push(`#EXT-X-BYTERANGE:${seg.size}@${seg.offset}`);
    lines.push(blobURI);
  }

  lines.push("#EXT-X-ENDLIST");
  return lines.join("\n") + "\n";
}

/**
 * Return the subset of segments that overlap the [startNs, endNs) window.
 * Segments are GOP-aligned so we include any segment whose time span
 * intersects the requested range — no sub-segment splitting.
 */
function filterSegments(
  segments: TrackSegment[],
  timescale: number,
  startNs?: number,
  endNs?: number,
): TrackSegment[] {
  if (startNs === undefined && endNs === undefined) {
    return segments;
  }

  // Convert nanoseconds to timescale ticks: ticks = ns * timescale / 1e9
  const startTicks = startNs !== undefined ? (startNs / 1e9) * timescale : 0;
  const endTicks = endNs !== undefined ? (endNs / 1e9) * timescale : Infinity;

  const result: TrackSegment[] = [];
  let cursor = 0; // running position in ticks

  for (const seg of segments) {
    const segEnd = cursor + seg.durationTicks;
    // Include segment if it overlaps [startTicks, endTicks)
    if (segEnd > startTicks && cursor < endTicks) {
      result.push(seg);
    }
    cursor = segEnd;
  }

  return result;
}

// ---------------------------------------------------------------------------
// place.stream.playback.getInitSegment
// ---------------------------------------------------------------------------

async function handleGetInitSegment(url: URL, env: Env): Promise<Response> {
  const parsed = requireURI(url);
  const track = url.searchParams.get("track");
  if (!track) {
    console.error(`[handleGetInitSegment] track parameter is required`);
    throw new XRPCError(400, "InvalidRequest", "track parameter is required");
  }

  const { meta } = await resolveVideo(parsed, env);
  const t = meta.tracks[track];
  if (!t) {
    console.error(`[handleGetInitSegment] Track ${track} not found`);
    throw new XRPCError(404, "TrackNotFound", `Track ${track} not found`);
  }

  const initUrl = `${env.S3_BUCKET_URL}/blobs/${t.initCid}.mp4`;
  console.log(`[getInitSegment] ${initUrl}`);
  const resp = await fetch(initUrl);
  if (!resp.ok) {
    console.error(`[handleGetInitSegment] Init segment not available`);
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
  const resp = await fetch(blobUrl, {
    headers,
    // @ts-ignore — CF-specific: bypass cache so Range headers are honored
    cf: { cacheTtl: 0, cacheEverything: false },
  });

  if (!resp.ok && resp.status !== 206) {
    console.error(`[handleGetVideoBlob] Blob not available for cid ${cid}`);
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
