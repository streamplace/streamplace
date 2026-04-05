import type { S3Client } from "@aws-sdk/client-s3";
import loadBlake3 from "blake3/browser-async";
import { bdaslCidFromDigest } from "./cid.js";
import {
  checkConnection,
  createS3Client,
  deleteObject,
  multipartCopy,
  MultipartUpload,
  putObject,
  type S3Config,
} from "./s3.js";
import { drainWrites, serveReads } from "./sab-io.js";

// -------------------------------------------------------------------------
// DOM helpers
// -------------------------------------------------------------------------

const $ = (id: string) => document.getElementById(id)!;
const show = (id: string) => $(id).classList.remove("hidden");
const hide = (id: string) => $(id).classList.add("hidden");

function setProgress(status: string, percent: number) {
  $("progress-status").textContent = status;
  ($("progress-fill") as HTMLDivElement).style.width = `${percent}%`;
}

function showError(msg: string) {
  $("error-msg").textContent = msg;
  show("error-section");
}

// -------------------------------------------------------------------------
// State
// -------------------------------------------------------------------------

let s3Client: S3Client | null = null;
let s3Config: S3Config | null = null;

// -------------------------------------------------------------------------
// Initialization
// -------------------------------------------------------------------------

// Check for SharedArrayBuffer support
if (typeof SharedArrayBuffer === "undefined") {
  show("compat-warning");
  ($("upload-btn") as HTMLButtonElement).disabled = true;
}

// Restore saved config
const saved = localStorage.getItem("muxl-admin-s3");
if (saved) {
  try {
    const cfg = JSON.parse(saved) as S3Config;
    (document.getElementById("s3-endpoint") as HTMLInputElement).value =
      cfg.endpoint;
    (document.getElementById("s3-bucket") as HTMLInputElement).value =
      cfg.bucket;
    (document.getElementById("s3-access-key") as HTMLInputElement).value =
      cfg.accessKeyId;
    (document.getElementById("s3-secret-key") as HTMLInputElement).value =
      cfg.secretAccessKey;
  } catch {}
}

// -------------------------------------------------------------------------
// File selection
// -------------------------------------------------------------------------

const fileInput = document.getElementById("file-input") as HTMLInputElement;
const uploadBtn = document.getElementById("upload-btn") as HTMLButtonElement;

fileInput.addEventListener("change", () => {
  const file = fileInput.files?.[0];
  if (file) {
    const sizeMB = (file.size / (1024 * 1024)).toFixed(1);
    $("file-info").textContent = `${file.name} (${sizeMB} MB)`;
    uploadBtn.disabled = false;
  } else {
    $("file-info").textContent = "";
    uploadBtn.disabled = true;
  }
});

// -------------------------------------------------------------------------
// Upload flow
// -------------------------------------------------------------------------

uploadBtn.addEventListener("click", async () => {
  const file = fileInput.files?.[0];
  if (!file) return;

  hide("error-section");
  hide("result-section");

  // Read S3 config
  s3Config = {
    endpoint: (
      document.getElementById("s3-endpoint") as HTMLInputElement
    ).value.trim(),
    bucket: (
      document.getElementById("s3-bucket") as HTMLInputElement
    ).value.trim(),
    accessKeyId: (
      document.getElementById("s3-access-key") as HTMLInputElement
    ).value.trim(),
    secretAccessKey: (
      document.getElementById("s3-secret-key") as HTMLInputElement
    ).value,
  };

  if (
    !s3Config.endpoint ||
    !s3Config.bucket ||
    !s3Config.accessKeyId ||
    !s3Config.secretAccessKey
  ) {
    showError("All S3 fields are required.");
    return;
  }

  // Save config (minus secret — store it for convenience in a demo tool)
  localStorage.setItem("muxl-admin-s3", JSON.stringify(s3Config));

  s3Client = createS3Client(s3Config);

  // Test connection
  show("progress-section");
  setProgress("Testing S3 connection...", 0);
  try {
    await checkConnection(s3Client, s3Config.bucket);
  } catch (e: any) {
    hide("progress-section");
    showError(`S3 connection failed: ${e.message}`);
    return;
  }

  uploadBtn.disabled = true;

  try {
    await runConversion(file, s3Client, s3Config.bucket);
  } catch (e: any) {
    hide("progress-section");
    showError(`Upload failed: ${e.message}`);
    uploadBtn.disabled = false;
  }
});

async function runConversion(file: File, client: S3Client, bucket: string) {
  setProgress("Starting WASM worker...", 0);

  // Spawn worker
  const worker = new Worker(new URL("./worker.ts", import.meta.url), {
    type: "module",
  });

  // Wait for worker to initialize WASM and send the SharedArrayBuffer
  const { buffer, readBufOffset, writeBufOffset } = await new Promise<{
    buffer: SharedArrayBuffer;
    readBufOffset: number;
    writeBufOffset: number;
  }>((resolve, reject) => {
    worker.onmessage = (e) => {
      if (e.data.type === "ready") resolve(e.data);
      else if (e.data.type === "error") reject(new Error(e.data.message));
    };
    worker.onerror = (e) => {
      reject(new Error(`Worker error: ${e.message}`));
    };
    worker.postMessage({ type: "start", fileSize: file.size });
  });

  // Wrap the SharedArrayBuffer in a Memory-like object for sab-io
  const memory = { buffer } as unknown as WebAssembly.Memory;

  // Set up the temp upload key
  const tempKey = `_tmp/${crypto.randomUUID()}.mp4`;
  const upload = new MultipartUpload(client, bucket, tempKey);
  await upload.start();

  // BLAKE3 hasher for CID computation — pass explicit WASM URL to avoid Vite transforming it
  const blake3 = await (loadBlake3 as unknown as (url: string) => Promise<{
    createHash: () => { update(data: Uint8Array): void; digest(): Uint8Array };
  }>)("/wasm/blake3_js_bg.wasm");
  const hasher = blake3.createHash();

  setProgress("Converting & uploading...", 0);

  // Run three concurrent loops:
  // 1. Serve read requests (WASM reads input file)
  // 2. Drain write chunks (WASM writes archive → BLAKE3 + S3)
  // 3. Wait for worker completion (tracks JSON)

  const tracksPromise = new Promise<string>((resolve, reject) => {
    worker.onmessage = (e) => {
      if (e.data.type === "done") resolve(e.data.tracksJson);
      else if (e.data.type === "error") reject(new Error(e.data.message));
    };
  });

  const readsPromise = serveReads(memory, readBufOffset, file, (bytesRead) => {
    const pct = Math.min(99, (bytesRead / file.size) * 100);
    setProgress(
      `Converting... ${(bytesRead / 1024 / 1024).toFixed(0)} MB read`,
      pct,
    );
  });

  const writesPromise = drainWrites(memory, writeBufOffset, async (chunk) => {
    hasher.update(chunk);
    await upload.addChunk(chunk);
  });

  // Wait for all to complete
  const [tracksJson, , totalWritten] = await Promise.all([
    tracksPromise,
    readsPromise,
    writesPromise,
  ]);

  worker.terminate();

  // Finalize the multipart upload
  setProgress("Finalizing upload...", 95);
  await upload.finish();

  // Compute CID
  const digest = hasher.digest();
  const archiveCid = bdaslCidFromDigest(digest);
  const totalSize = totalWritten;

  // Copy to final key
  setProgress("Copying to final key...", 96);
  await multipartCopy(client, bucket, tempKey, `${archiveCid}.mp4`, totalSize);

  // Delete temp
  await deleteObject(client, bucket, tempKey);

  // Parse tracks and upload init segments + metadata
  setProgress("Uploading metadata...", 98);
  const tracks = JSON.parse(tracksJson) as Array<{
    trackId: number;
    trackType: string;
    codec: string;
    timescale: number;
    initCid: string;
    blobCid: string;
    blobSize: number;
    segments: Array<{
      offset: number;
      size: number;
      durationTicks: number;
      sampleCount: number;
    }>;
    width: number;
    height: number;
    channels: number;
    sampleRate: number;
  }>;

  // Fill in blob CID/size (WASM left these blank)
  for (const t of tracks) {
    t.blobCid = archiveCid;
    t.blobSize = totalSize;
  }

  // Build and upload metadata JSON (same format as muxl hls output)
  const metadata = {
    blobCid: archiveCid,
    blobSize: totalSize,
    tracks: Object.fromEntries(tracks.map((t) => [String(t.trackId), t])),
  };
  await putObject(
    client,
    bucket,
    `${archiveCid}.json`,
    JSON.stringify(metadata, null, 2),
    "application/json",
  );

  // Upload init segments (we need to read them from the write stream)
  // TODO: init segments are written after the archive data in the write stream.
  // For now, we'd need to collect them separately. The WASM convert_flat_mp4
  // function writes them after the archive. We'll handle this in a follow-up.
  // For now, init segments can be uploaded by reading them from the metadata
  // tracks' initCid fields — they're already on disk if using muxl hls CLI.

  // Show results
  hide("progress-section");
  show("result-section");
  $("result-cid").innerHTML =
    `<strong>Archive CID:</strong> <code>${archiveCid}</code>`;

  let tracksHtml = "";
  for (const t of tracks) {
    const durSec = t.segments.reduce(
      (s, seg) => s + seg.durationTicks / t.timescale,
      0,
    );
    tracksHtml += `<div class="track-info">Track ${t.trackId}: ${t.trackType} (${t.codec}) — ${t.segments.length} segments, ${durSec.toFixed(1)}s</div>`;
  }
  $("result-tracks").innerHTML = tracksHtml;

  uploadBtn.disabled = false;
}

// Reset button
$("another-btn").addEventListener("click", () => {
  hide("result-section");
  hide("error-section");
  fileInput.value = "";
  $("file-info").textContent = "";
  uploadBtn.disabled = true;
});
