import * as tus from "tus-js-client";

// Module-level upload manager. TUS uploads live here — outside the React
// tree — so navigating away from the upload screen (which unmounts it) no
// longer aborts an in-flight upload. The floating UploadProgressIndicator in
// the app shell subscribes to this store and renders progress until every
// upload finishes.

export type UploadJob = {
  id: string;
  tid: string;
  filename: string;
  status: "uploading" | "done" | "error";
  bytesSent: number;
  bytesTotal: number;
  error?: string;
};

// How long a completed upload lingers in the indicator before auto-dismissing.
const DONE_LINGER_MS = 4000;

let jobs: UploadJob[] = [];
let nextId = 1;
const listeners = new Set<() => void>();
const uploads = new Map<string, tus.Upload>();

function emit() {
  listeners.forEach((l) => l());
}

function patchJob(id: string, patch: Partial<UploadJob>) {
  jobs = jobs.map((j) => (j.id === id ? { ...j, ...patch } : j));
  emit();
}

// Warn before the tab closes while uploads are still running (web only).
function beforeUnload(e: BeforeUnloadEvent) {
  e.preventDefault();
}

function syncBeforeUnload() {
  if (typeof window === "undefined" || !window.addEventListener) return;
  if (jobs.some((j) => j.status === "uploading")) {
    window.addEventListener("beforeunload", beforeUnload);
  } else {
    window.removeEventListener("beforeunload", beforeUnload);
  }
}

export function subscribeUploads(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getUploads(): UploadJob[] {
  return jobs;
}

export function startUpload({
  file,
  uploadUrl,
  uploadToken,
  tid,
}: {
  file: File;
  uploadUrl: string;
  uploadToken: string;
  tid: string;
}): string {
  const id = `upload-${nextId++}`;
  jobs = [
    ...jobs,
    {
      id,
      tid,
      filename: file.name,
      status: "uploading",
      bytesSent: 0,
      bytesTotal: file.size,
    },
  ];
  emit();
  syncBeforeUnload();

  let retried = false;
  const params: tus.UploadOptions = {
    uploadUrl,
    retryDelays: [0, 1000, 3000, 5000],
    headers: { Authorization: `Bearer ${uploadToken}` },
    metadata: { filename: file.name, filetype: file.type },
    onError: (err) => {
      if (!retried) {
        retried = true;
        // <1mb for default nginx proxy settings
        params.chunkSize = 800000;
        doTry();
      } else {
        console.error("upload failed", err);
        uploads.delete(id);
        patchJob(id, {
          status: "error",
          error: err instanceof Error ? err.message : String(err),
        });
        syncBeforeUnload();
      }
    },
    onProgress: (bytesSent, bytesTotal) => {
      patchJob(id, { bytesSent, bytesTotal });
    },
    onSuccess: () => {
      uploads.delete(id);
      patchJob(id, { status: "done", bytesSent: file.size });
      syncBeforeUnload();
      setTimeout(() => dismissUpload(id), DONE_LINGER_MS);
    },
  };
  const doTry = () => {
    const upload = new tus.Upload(file, params);
    uploads.set(id, upload);
    upload.start();
  };
  doTry();
  return id;
}

export function cancelUpload(id: string) {
  uploads.get(id)?.abort();
  uploads.delete(id);
  dismissUpload(id);
  syncBeforeUnload();
}

export function dismissUpload(id: string) {
  if (!jobs.some((j) => j.id === id)) return;
  jobs = jobs.filter((j) => j.id !== id);
  emit();
}
