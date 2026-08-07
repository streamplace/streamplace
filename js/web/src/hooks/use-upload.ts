// Upload state machine for VOD uploads. Mirrors the RN upload flow but
// simplified for the web (no expo-document-picker, no expo-file-system).
import { useCallback, useEffect, useRef, useState } from "react";
import type { PlaceStreamVideo } from "streamplace";
import * as tus from "tus-js-client";
import { usePDSAgent } from "../lib/store/hooks";

// ── types ────────────────────────────────────────────────────────────────────

type TrackRef = { uri: string; cid: string };

export type UploadPhase =
  | { kind: "idle" }
  | { kind: "creating" }
  | { kind: "uploading"; pct: number; bytesSent: number; bytesTotal: number }
  | {
      kind: "processing";
      uploadId: string;
      serverStatus?: "pending" | "processing";
      progress?: number;
    }
  | { kind: "ready"; uploadId: string; tracks: TrackRef[]; durationMs: number }
  | { kind: "publishing" }
  | { kind: "done"; videoUri: string }
  | { kind: "error"; message: string };

export interface UploadMetadata {
  title: string;
  description: string;
  tags: string[];
  warnings: Set<string>;
  license: string;
  thumbnail?: Blob;
}

const POLL_INTERVAL_MS = 3000;

// ── helpers ───────────────────────────────────────────────────────────────────

export function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

// Returns an error string if the file doesn't look like a video, null if ok.
// Reads the first 12 bytes and matches magic signatures for common containers.
export async function validateVideoFile(file: File): Promise<string | null> {
  const buf = await file.slice(0, 12).arrayBuffer();
  const b = new Uint8Array(buf);

  const str = (offset: number, len: number) =>
    String.fromCharCode(...Array.from(b.slice(offset, offset + len)));

  // MP4 / MOV / M4V — "ftyp" box at offset 4
  if (str(4, 4) === "ftyp") return null;
  // WebM / MKV — EBML magic
  if (b[0] === 0x1a && b[1] === 0x45 && b[2] === 0xdf && b[3] === 0xa3)
    return null;
  // AVI — "RIFF" header with "AVI " chunk
  if (str(0, 4) === "RIFF" && str(8, 4) === "AVI ") return null;
  // OGG (video: OGV, Theora)
  if (str(0, 4) === "OggS") return null;
  // FLV
  if (str(0, 3) === "FLV") return null;
  // MPEG-TS — 188-byte packets starting with sync byte 0x47
  if (b[0] === 0x47) return null;

  return "File doesn't appear to be a supported video format (MP4, WebM, MKV, MOV, AVI, OGG, FLV, MPEG-TS).";
}

// ── hook ──────────────────────────────────────────────────────────────────────

export function useUpload() {
  const agent = usePDSAgent();
  const uploadRef = useRef<tus.Upload | null>(null);
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [phase, setPhase] = useState<UploadPhase>({ kind: "idle" });
  const [file, setFile] = useState<File | null>(null);

  // metadata form — always editable
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [thumbnail, setThumbnail] = useState<Blob | undefined>(undefined);
  const [thumbnailUrl, setThumbnailUrl] = useState<string | undefined>(
    undefined,
  );
  const [warnings, setWarnings] = useState<Set<string>>(new Set());
  const [license, setLicense] = useState<string>(
    "place.stream.metadata.contentRights#all-rights-reserved",
  );

  // cleanup on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearTimeout(pollRef.current);
      uploadRef.current?.abort();
    };
  }, []);

  // clean up blob URLs when thumbnail changes or unmounts
  useEffect(() => {
    return () => {
      if (thumbnailUrl) URL.revokeObjectURL(thumbnailUrl);
    };
  }, [thumbnailUrl]);

  // ── file selection ────────────────────────────────────────────────────────

  const selectFile = useCallback((f: File | null) => {
    setFile(f);
    if (f) setTitle((t) => t || f.name.replace(/\.[^.]+$/, ""));
    setPhase({ kind: "idle" });
  }, []);

  // ── thumbnail ─────────────────────────────────────────────────────────────

  const selectThumbnail = useCallback((f: File | null) => {
    if (f) {
      const blob = new Blob([f], { type: f.type });
      setThumbnail(blob);
      setThumbnailUrl(URL.createObjectURL(blob));
    }
  }, []);

  const removeThumbnail = useCallback(() => {
    setThumbnail(undefined);
    setThumbnailUrl(undefined);
  }, []);

  // ── tags ──────────────────────────────────────────────────────────────────

  const addTag = useCallback(
    (tag: string) => {
      const t = tag.trim();
      if (t && !tags.includes(t) && tags.length < 10) {
        setTags([...tags, t]);
      }
      setTagInput("");
    },
    [tags],
  );

  const removeTag = useCallback((tag: string) => {
    setTags((prev) => prev.filter((t) => t !== tag));
  }, []);

  // ── content warnings ──────────────────────────────────────────────────────

  const toggleWarning = useCallback((value: string) => {
    setWarnings((prev) => {
      const next = new Set(prev);
      next.has(value) ? next.delete(value) : next.add(value);
      return next;
    });
  }, []);

  // ── polling ───────────────────────────────────────────────────────────────

  const pollStatus = useCallback(
    (uploadId: string) => {
      if (!agent) return;
      const check = async () => {
        try {
          const res = await agent.place.stream.media.getUploadStatus({
            uploadId,
          });
          const data = res.data;

          if (data.status === "done" && data.tracks) {
            setPhase({
              kind: "ready",
              uploadId,
              tracks: data.tracks,
              durationMs: data.durationMs ?? 0,
            });
            return;
          }
          if (data.status === "error") {
            setPhase({
              kind: "error",
              message: data.error ?? "Processing failed",
            });
            return;
          }
          setPhase({
            kind: "processing",
            uploadId,
            serverStatus: data.status as "pending" | "processing",
            progress: data.progress,
          });
          pollRef.current = setTimeout(check, POLL_INTERVAL_MS);
        } catch {
          pollRef.current = setTimeout(check, POLL_INTERVAL_MS);
        }
      };
      check();
    },
    [agent],
  );

  // ── upload ────────────────────────────────────────────────────────────────

  const startUpload = useCallback(async () => {
    if (!agent || !file) return;
    if (!agent.did) {
      setPhase({ kind: "error", message: "Not logged in" });
      return;
    }
    const validationError = await validateVideoFile(file);
    if (validationError) {
      setPhase({ kind: "error", message: validationError });
      return;
    }
    const mimeType = file.type.startsWith("video/") ? file.type : "video/mp4";
    setPhase({ kind: "creating" });
    try {
      const res = await agent.place.stream.media.createUpload({
        size: file.size,
        mimeType,
        filename: file.name,
      });
      if (!res.success) throw new Error("createUpload failed");
      const { uploadUrl, uploadToken, uploadId } = res.data;

      await new Promise<void>((resolve, reject) => {
        const upload = new tus.Upload(file, {
          uploadUrl,
          retryDelays: [0, 1000, 3000, 5000],
          headers: { Authorization: `Bearer ${uploadToken}` },
          metadata: { filename: file.name, filetype: file.type },
          onError: reject,
          onProgress(bytesSent, bytesTotal) {
            setPhase({
              kind: "uploading",
              pct: bytesTotal > 0 ? (bytesSent / bytesTotal) * 100 : 0,
              bytesSent,
              bytesTotal,
            });
          },
          onSuccess: () => resolve(),
        });
        uploadRef.current = upload;
        upload.start();
      });

      uploadRef.current = null;
      setPhase({ kind: "processing", uploadId });
      pollStatus(uploadId);
    } catch (err) {
      uploadRef.current = null;
      setPhase({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }, [agent, file, pollStatus]);

  const cancelUpload = useCallback(() => {
    if (pollRef.current) clearTimeout(pollRef.current);
    uploadRef.current?.abort();
    uploadRef.current = null;
    setPhase({ kind: "idle" });
  }, []);

  // ── publish ───────────────────────────────────────────────────────────────

  const publish = useCallback(async () => {
    if (phase.kind !== "ready" || !agent || !agent.did) return;
    setPhase({ kind: "publishing" });
    try {
      const { tracks, durationMs } = phase;

      const record: PlaceStreamVideo.Record = {
        $type: "place.stream.video",
        title: title.trim() || file?.name || "Untitled",
        createdAt: new Date().toISOString(),
        durationMs,
        source: {
          $type: "place.stream.media.defs#sourceTracks",
          tracks: tracks.map((t) => ({
            $type: "com.atproto.repo.strongRef",
            uri: t.uri,
            cid: t.cid,
          })),
        },
      };

      if (description.trim()) record.description = description.trim();
      if (tags.length > 0) record.tags = tags;

      if (warnings.size > 0) {
        const cw: Record<string, boolean> = {};
        for (const w of warnings) {
          const key = w.split("#")[1];
          if (key) cw[key] = true;
        }
        record.contentWarnings = {
          $type: "place.stream.metadata.contentWarnings",
          ...cw,
        } as any;
      }

      if (
        license &&
        license !== "place.stream.metadata.contentRights#all-rights-reserved"
      ) {
        const licenseKey = license.split("#")[1];
        record.contentRights = {
          $type: "place.stream.metadata.contentRights",
          license: { $type: license } as any,
          [licenseKey]: { $type: license } as any,
        } as any;
      }

      if (thumbnail) {
        try {
          const blobRes = await agent.uploadBlob(thumbnail, {
            encoding: thumbnail.type || "image/jpeg",
          });
          if (blobRes.success) record.thumb = blobRes.data.blob as any;
        } catch {
          // thumbnail failure is non-fatal
        }
      }

      const res = await agent.place.stream.media.publishVideo({
        uploadId: phase.uploadId,
        record,
      });

      if (!res.success) throw new Error("publishVideo failed");
      setPhase({ kind: "done", videoUri: res.data.uri });
    } catch (err) {
      setPhase({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }, [
    phase,
    agent,
    title,
    description,
    tags,
    thumbnail,
    warnings,
    license,
    file,
  ]);

  // ── derived state ─────────────────────────────────────────────────────────

  const isUploading =
    phase.kind === "creating" ||
    phase.kind === "uploading" ||
    phase.kind === "processing";
  const isPublishing = phase.kind === "publishing";
  const canUpload = !!file && (phase.kind === "idle" || phase.kind === "error");
  const canPublish = phase.kind === "ready" && !!title.trim();

  // ── direct setters (for edit mode) ─────────────────────────────────────

  const setTagsDirectly = useCallback((newTags: string[]) => {
    setTags(newTags);
  }, []);

  const setWarningsDirectly = useCallback((newWarnings: Set<string>) => {
    setWarnings(newWarnings);
  }, []);

  // ── reset ─────────────────────────────────────────────────────────────────

  const reset = useCallback(() => {
    setFile(null);
    setTitle("");
    setDescription("");
    setTags([]);
    setTagInput("");
    setThumbnail(undefined);
    setThumbnailUrl(undefined);
    setWarnings(new Set());
    setLicense("place.stream.metadata.contentRights#all-rights-reserved");
    setPhase({ kind: "idle" });
  }, []);

  return {
    // state
    phase,
    file,
    title,
    description,
    tags,
    tagInput,
    thumbnail,
    thumbnailUrl,
    warnings,
    license,
    // derived
    isUploading,
    isPublishing,
    canUpload,
    canPublish,
    // actions
    selectFile,
    selectThumbnail,
    removeThumbnail,
    setTitle,
    setDescription,
    setTagInput,
    addTag,
    removeTag,
    toggleWarning,
    setTagsDirectly,
    setWarningsDirectly,
    setLicense,
    startUpload,
    cancelUpload,
    publish,
    reset,
  };
}
