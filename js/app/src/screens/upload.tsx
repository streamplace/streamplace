import { useRoute } from "@react-navigation/native";
import {
  Admonition,
  Button,
  Checkbox,
  MenuContainer,
  MenuGroup,
  MenuLabel,
  MenuSeparator,
  Select,
  Text,
  Tooltip,
  useBetaStatus,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
import {
  CONTENT_WARNINGS,
  LICENSE_OPTIONS,
} from "@streamplace/components/src/lib/metadata-constants";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import ActivityPicker from "components/activity-picker";
import AQLink from "components/aqlink";
import Loading from "components/loading/loading";
import BetaAccessGate from "components/upload/beta-access-gate";
import { Image } from "expo-image";
import {
  AlertCircle,
  ArrowUp,
  CheckCircle2,
  ImagePlus,
  LoaderCircle,
  Video,
  X,
} from "lucide-react-native";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Animated,
  Pressable,
  ScrollView,
  TextInput,
  useWindowDimensions,
} from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";
import type {
  PlaceStreamLivestream,
  PlaceStreamVodDraftDefs,
  PlaceStreamVodDraftVideo,
} from "streamplace";
import * as tus from "tus-js-client";

// ── types ────────────────────────────────────────────────────────────────────

type TrackRef = { uri: string; cid: string };

type UploadPhase =
  | { kind: "idle" }
  | { kind: "creating" }
  | { kind: "uploading"; pct: number; bytesSent: number; bytesTotal: number }
  | {
      kind: "processing";
      uploadId: string;
      serverStatus?: "pending" | "processing";
      progress?: number;
    }
  // With draft VODs the upload flow no longer polls or publishes: once the
  // bytes are uploaded the server creates a draft automatically, and the user
  // finishes from the Drafts tab. `ready`/`publishing` are retained for type
  // compatibility but are no longer reached by the upload state machine.
  | { kind: "ready"; uploadId: string; tracks: TrackRef[]; durationMs: number }
  | { kind: "publishing" }
  | { kind: "done" }
  | { kind: "error"; message: string };

// ── helpers ───────────────────────────────────────────────────────────────────

function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

// Returns an error string if the file doesn't look like a video, null if ok.
// Reads the first 12 bytes and matches magic signatures for common containers.
async function validateVideoFile(file: File): Promise<string | null> {
  const buf = await file.slice(0, 12).arrayBuffer();
  const b = new Uint8Array(buf);

  const eq = (offset: number, ...bytes: number[]) =>
    bytes.every((v, i) => b[offset + i] === v);
  const str = (offset: number, len: number) =>
    String.fromCharCode(...Array.from(b.slice(offset, offset + len)));

  // MP4 / MOV / M4V — "ftyp" box at offset 4
  if (str(4, 4) === "ftyp") return null;
  // WebM / MKV — EBML magic
  if (eq(0, 0x1a, 0x45, 0xdf, 0xa3)) return null;
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

// ── screen ───────────────────────────────────────────────────────────────────

const BETA_FEATURE = "vod";

export default function UploadScreen() {
  const agent = usePDSAgent();
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const {
    status: betaStatus,
    loading: betaLoading,
    markRequested,
  } = useBetaStatus(BETA_FEATURE);
  const { theme, zero: zt } = useTheme();
  const { width } = useWindowDimensions();
  const isWide = width > 800;

  // file
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const uploadRef = useRef<tus.Upload | null>(null);
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const processingAnim = useRef(new Animated.Value(0)).current;
  const [file, setFile] = useState<File | null>(null);

  // upload state machine
  const [phase, setPhase] = useState<UploadPhase>({ kind: "idle" });

  // indeterminate progress bar animation
  useEffect(() => {
    if (
      phase.kind === "creating" ||
      phase.kind === "processing" ||
      phase.kind === "publishing"
    ) {
      const anim = Animated.loop(
        Animated.sequence([
          Animated.timing(processingAnim, {
            toValue: 1,
            duration: 1200,
            useNativeDriver: false,
          }),
          Animated.timing(processingAnim, {
            toValue: 0,
            duration: 1200,
            useNativeDriver: false,
          }),
        ]),
      );
      anim.start();
      return () => anim.stop();
    } else {
      processingAnim.setValue(0);
    }
  }, [phase.kind, processingAnim]);

  // metadata form — always editable
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [activity, setActivity] =
    useState<PlaceStreamLivestream.Record["activity"]>(undefined);
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [thumbnail, setThumbnail] = useState<Blob | undefined>(undefined);
  const [thumbnailUrl, setThumbnailUrl] = useState<string | undefined>(
    undefined,
  );
  const thumbnailInputRef = useRef<HTMLInputElement | null>(null);
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

  // clean up old blob URLs when thumbnail changes or unmounts
  useEffect(() => {
    return () => {
      if (thumbnailUrl) URL.revokeObjectURL(thumbnailUrl);
    };
  }, [thumbnailUrl]);

  const pickFile = useCallback(() => fileInputRef.current?.click(), []);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0] ?? null;
      setFile(f);
      if (f) setTitle((t) => t || f.name.replace(/\.[^.]+$/, ""));
      setPhase({ kind: "idle" });
      e.target.value = "";
    },
    [],
  );

  const toggleWarning = useCallback((value: string) => {
    setWarnings((prev) => {
      const next = new Set(prev);
      next.has(value) ? next.delete(value) : next.add(value);
      return next;
    });
  }, []);

  // thumbnail handlers
  const handleThumbnailSelect = useCallback(() => {
    thumbnailInputRef.current?.click();
  }, []);

  const handleThumbnailChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0] ?? null;
      if (f) {
        const blob = new Blob([f], { type: f.type });
        setThumbnail(blob);
        setThumbnailUrl(URL.createObjectURL(blob));
      }
      e.target.value = "";
    },
    [],
  );

  const handleThumbnailRemove = useCallback(() => {
    setThumbnail(undefined);
    setThumbnailUrl(undefined);
  }, []);

  const [chunkSize, setChunkSize] = useState<number | undefined>(undefined);

  // ── upload ───────────────────────────────────────────────────────────────

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
      const { uploadUrl, uploadToken } = res.data;

      await new Promise<void>((resolve, reject) => {
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
              console.log(err);
              reject(err);
            }
          },
          onProgress(bytesSent, bytesTotal) {
            setPhase({
              kind: "uploading",
              pct: bytesTotal > 0 ? (bytesSent / bytesTotal) * 100 : 0,
              bytesSent,
              bytesTotal,
            });
          },
          onSuccess: () => resolve(),
        };
        const doTry = () => {
          const upload = new tus.Upload(file, params);
          uploadRef.current = upload;
          upload.start();
        };
        doTry();
      });

      uploadRef.current = null;
      // The server creates a draft VOD automatically once the TUS upload
      // finishes processing. The client no longer polls getUploadStatus or
      // calls publishVideo — the user publishes from the Drafts tab instead.
      setPhase({ kind: "done" });
    } catch (err) {
      uploadRef.current = null;
      setPhase({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }, [agent, file, chunkSize]);

  const cancelUpload = useCallback(() => {
    if (pollRef.current) clearTimeout(pollRef.current);
    uploadRef.current?.abort();
    uploadRef.current = null;
    setPhase({ kind: "idle" });
  }, []);

  // ── derived state ─────────────────────────────────────────────────────────

  const isUploading =
    phase.kind === "creating" ||
    phase.kind === "uploading" ||
    phase.kind === "processing";
  const canUpload = !!file && (phase.kind === "idle" || phase.kind === "error");

  // ── VOD manager ────────────────────────────────────────────────────────────

  const [managerMode, setManagerMode] = useState<
    "upload" | "videos" | "livestreams" | "drafts"
  >("upload");
  const [userVideos, setUserVideos] = useState<any[]>([]);
  const [livestreams, setLivestreams] = useState<any[]>([]);
  const [livestreamsLoading, setLivestreamsLoading] = useState(false);
  // Per-livestream finalize state. With draft VODs, finalize no longer polls
  // or publishes: it just kicks off the server task and flips the row to
  // "done" so the user knows to find the draft in the Drafts tab.
  const [finalizing, setFinalizing] = useState<
    Record<string, "finalizing" | "done" | "error">
  >({});
  // Livestream URI -> draft ats:// URI, recorded when finalizeLivestream
  // returns so the row can offer a deep-link to the Drafts tab.
  const [finalizedDraftUris, setFinalizedDraftUris] = useState<
    Record<string, string>
  >({});
  const [editingVideoUri, setEditingVideoUri] = useState<string | undefined>(
    undefined,
  );
  const [updating, setUpdating] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // ── drafts ──────────────────────────────────────────────────────────────────
  const [drafts, setDrafts] = useState<PlaceStreamVodDraftDefs.DraftView[]>([]);
  const [draftsLoading, setDraftsLoading] = useState(false);
  // When editing a draft, holds the draft's ats:// URI so the save handler
  // knows to call updateDraft instead of putRecord.
  const [editingDraftUri, setEditingDraftUri] = useState<string | undefined>(
    undefined,
  );
  const [draftAction, setDraftAction] = useState<
    Record<string, "publishing" | "deleting" | "error">
  >({});
  // Optimistically-recorded published video URIs: publishDraft returns the new
  // place.stream.video URI, which we surface as a "View VOD" link before the
  // drafts list refresh removes the row server-side.
  const [publishedFromDrafts, setPublishedFromDrafts] = useState<
    Record<string, string>
  >({});

  const fetchVideos = useCallback(async () => {
    if (!agent || !agent.did) return;
    try {
      const res = await agent.place.stream.media.getVideoList({
        repo: agent.did,
      });
      setUserVideos(res.data.videos || []);
    } catch (err) {
      console.error("Failed to fetch videos", err);
    }
  }, [agent]);

  const fetchLivestreams = useCallback(async () => {
    if (!agent || !agent.did) return;
    setLivestreamsLoading(true);
    try {
      const res = await agent.com.atproto.repo.listRecords({
        repo: agent.did,
        collection: "place.stream.livestream",
        limit: 100,
      });
      const records = (res.data.records ?? []).slice().sort((a, b) => {
        const at = ((a.value as any)?.createdAt as string) ?? "";
        const bt = ((b.value as any)?.createdAt as string) ?? "";
        return bt.localeCompare(at); // newest first
      });
      setLivestreams(records);
    } catch (err) {
      console.error("Failed to fetch livestreams", err);
    } finally {
      setLivestreamsLoading(false);
    }
  }, [agent]);

  const fetchDrafts = useCallback(async () => {
    if (!agent || !agent.did) return;
    setDraftsLoading(true);
    try {
      const res = await agent.place.stream.vod.listDrafts({ limit: 100 });
      setDrafts(res.data.drafts || []);
    } catch (err) {
      console.error("Failed to fetch drafts", err);
    } finally {
      setDraftsLoading(false);
    }
  }, [agent]);

  // Cross-reference the user's published videos against their livestreams via
  // the video record's `connections` array: a finalized livestream is one some
  // VOD links back to. media.publishVideo preserves client-supplied
  // connections, so the finalize flow below stamps the source livestream there.
  const livestreamToVideo = useMemo(() => {
    const map = new Map<string, string>();
    for (const v of userVideos) {
      const rec = (v.record?.value || v.record || {}) as any;
      for (const c of rec.connections ?? []) {
        const uri = c?.ref?.uri;
        if (
          typeof uri === "string" &&
          uri.includes("/place.stream.livestream/")
        ) {
          map.set(uri, v.uri);
        }
      }
    }
    return map;
  }, [userVideos]);

  useEffect(() => {
    // The livestreams tab also needs the videos list to compute which streams
    // have already been finalized.
    if (managerMode === "videos" || managerMode === "livestreams") {
      fetchVideos();
    }
    if (managerMode === "livestreams") {
      fetchLivestreams();
    }
    if (managerMode === "drafts") {
      fetchDrafts();
    }
  }, [managerMode, fetchVideos, fetchLivestreams, fetchDrafts]);

  // Finalize a single livestream into a draft VOD. With draft VODs the
  // client no longer polls getUploadStatus or calls publishVideo: the server
  // kicks off the remux/processing task and creates a draft record (status
  // "processing" → "ready") that the user publishes later from the Drafts tab.
  // We just call finalizeLivestream and flip the row to a "done" state that
  // points the user at the Drafts tab.
  const finalizeLivestreamRow = useCallback(
    async (ls: { uri: string; cid: string; value: any }) => {
      if (!agent || !agent.did) return;
      setFinalizing((m) => ({ ...m, [ls.uri]: "finalizing" }));
      try {
        const res = await agent.place.stream.media.finalizeLivestream({
          livestream: ls.uri,
        });
        if (!res.success) throw new Error("finalizeLivestream failed");
        // Record the draft URI so the row can offer a deep-link to the
        // Drafts tab (optional affordance).
        if (res.data.draftUri) {
          setFinalizedDraftUris((m) => ({
            ...m,
            [ls.uri]: res.data.draftUri as string,
          }));
        }
        setFinalizing((m) => ({ ...m, [ls.uri]: "done" }));
      } catch (err) {
        console.error("Failed to finalize livestream", err);
        setFinalizing((m) => ({ ...m, [ls.uri]: "error" }));
      }
    },
    [agent],
  );

  const handleSelectVideo = useCallback((video: any) => {
    const rec = video.record?.value || video.record || {};
    setEditingVideoUri(video.uri);
    setTitle(rec.title || "");
    setDescription(rec.description || "");
    setActivity(rec.activity || undefined);
    setTags(rec.tags || []);
    setThumbnail(undefined);
    setThumbnailUrl(undefined);
    // Clear content warnings
    const cw = rec.contentWarnings?.warnings || [];
    setWarnings(new Set(cw));
    // Set license
    const rights = rec.contentRights || {};
    setLicense(
      rights.license?.$type ||
        "place.stream.metadata.contentRights#all-rights-reserved",
    );
    setManagerMode("upload");
  }, []);

  const handleUpdateVideo = useCallback(async () => {
    if (!agent || !agent.did || !editingVideoUri) return;
    setUpdating(true);
    try {
      const record: Record<string, any> = {
        $type: "place.stream.video",
        title: title.trim(),
        source: {}, // placeholder, will be merged with existing
        durationMs: 0, // placeholder
      };
      // Fetch existing record to preserve source/duration
      const existing = await agent.com.atproto.repo.getRecord({
        repo: agent.did,
        collection: "place.stream.video",
        rkey: editingVideoUri.split("/").pop()!,
      });
      const existingRec = existing.data.value as any;
      record.source = existingRec.source;
      record.durationMs = existingRec.durationMs;
      // Preserve the original creation timestamp on edit; fall back for
      // records that predate the createdAt field.
      record.createdAt = existingRec.createdAt || new Date().toISOString();
      if (description.trim()) record.description = description.trim();
      if (activity) record.activity = activity;
      if (tags.length > 0) record.tags = tags;
      if (warnings.size > 0) {
        record.contentWarnings = {
          $type: "place.stream.metadata.contentWarnings",
          warnings: [...warnings],
        };
      }
      if (
        license &&
        license !== "place.stream.metadata.contentRights#all-rights-reserved"
      ) {
        record.contentRights = {
          $type: "place.stream.metadata.contentRights",
          license: license,
        };
      }
      if (thumbnail) {
        try {
          const blobRes = await agent.uploadBlob(thumbnail, {
            encoding: thumbnail.type || "image/jpeg",
          });
          if (blobRes.success) record.thumb = blobRes.data.blob;
        } catch {
          // thumbnail is non-fatal
        }
      }
      await agent.com.atproto.repo.putRecord({
        repo: agent.did,
        collection: "place.stream.video",
        rkey: editingVideoUri.split("/").pop()!,
        record: record as any,
      });
      setEditingVideoUri(undefined);
      setFile(null);
      setPhase({ kind: "idle" });
    } catch (err) {
      console.error("Failed to update video", err);
    } finally {
      setUpdating(false);
    }
  }, [
    agent,
    editingVideoUri,
    title,
    description,
    activity,
    tags,
    warnings,
    license,
    thumbnail,
  ]);

  const handleDeleteVideo = useCallback(async () => {
    if (!agent || !editingVideoUri) return;
    setDeleting(true);
    try {
      await agent.com.atproto.repo.deleteRecord({
        repo: agent.did!,
        collection: "place.stream.video",
        rkey: editingVideoUri.split("/").pop()!,
      });
      setEditingVideoUri(undefined);
      setFile(null);
      setPhase({ kind: "idle" });
      setManagerMode("videos");
    } catch (err) {
      console.error("Failed to delete video", err);
    } finally {
      setDeleting(false);
    }
  }, [agent, editingVideoUri]);

  // ── draft handlers ──────────────────────────────────────────────────────────

  // Load a draft's metadata into the editor form and switch to the upload tab,
  // mirroring handleSelectVideo. editingDraftUri tells the save handler to
  // call updateDraft instead of putRecord.
  const handleSelectDraft = useCallback(
    (draft: PlaceStreamVodDraftDefs.DraftView) => {
      const rec = draft.record as any;
      setEditingDraftUri(draft.uri);
      setEditingVideoUri(undefined);
      setTitle(rec.title || "");
      setDescription(rec.description || "");
      setActivity(rec.activity || undefined);
      setTags(rec.tags || []);
      setThumbnail(undefined);
      setThumbnailUrl(
        rec.thumb
          ? `https://cdn.stream.place/thumb/${rec.thumb.ref?.$link || rec.thumb.cid || ""}`
          : undefined,
      );
      const cw = rec.contentWarnings?.warnings || [];
      setWarnings(new Set(cw));
      const rights = rec.contentRights || {};
      setLicense(
        rights.license?.$type ||
          rights.license ||
          "place.stream.metadata.contentRights#all-rights-reserved",
      );
      setManagerMode("upload");
    },
    [],
  );

  // Save edits to a draft via updateDraft (not putRecord). The server owns
  // source/duration/status; we only send the editable fields.
  const handleUpdateDraft = useCallback(async () => {
    if (!agent || !editingDraftUri) return;
    setUpdating(true);
    try {
      const params: Record<string, any> = {
        uri: editingDraftUri,
        title: title.trim() || "Untitled",
      };
      if (description.trim()) params.description = description.trim();
      if (activity) params.activity = activity;
      if (tags.length > 0) params.tags = tags;
      if (warnings.size > 0) {
        params.contentWarnings = {
          $type: "place.stream.metadata.contentWarnings",
          warnings: [...warnings],
        };
      }
      if (
        license &&
        license !== "place.stream.metadata.contentRights#all-rights-reserved"
      ) {
        params.contentRights = {
          $type: "place.stream.metadata.contentRights",
          license: license,
        };
      }
      if (thumbnail) {
        try {
          const blobRes = await agent.uploadBlob(thumbnail, {
            encoding: thumbnail.type || "image/jpeg",
          });
          if (blobRes.success) params.thumb = blobRes.data.blob;
        } catch {
          // thumbnail failure is non-fatal
        }
      }
      await agent.place.stream.vod.updateDraft(params as any);
      setEditingDraftUri(undefined);
      setFile(null);
      setPhase({ kind: "idle" });
      setManagerMode("drafts");
    } catch (err) {
      console.error("Failed to update draft", err);
    } finally {
      setUpdating(false);
    }
  }, [
    agent,
    editingDraftUri,
    title,
    description,
    activity,
    tags,
    thumbnail,
    warnings,
    license,
  ]);

  // Publish a ready draft. On success the draft is deleted server-side; we
  // record the new video URI optimistically and refresh the list.
  const handlePublishDraft = useCallback(
    async (uri: string) => {
      if (!agent) return;
      setDraftAction((m) => ({ ...m, [uri]: "publishing" }));
      try {
        const res = await agent.place.stream.vod.publishDraft({ uri });
        setPublishedFromDrafts((m) => ({ ...m, [uri]: res.data.videoUri }));
        setDrafts((prev) => prev.filter((d) => d.uri !== uri));
      } catch (err) {
        console.error("Failed to publish draft", err);
        setDraftAction((m) => ({ ...m, [uri]: "error" }));
      }
    },
    [agent],
  );

  const handleDeleteDraft = useCallback(
    async (uri: string) => {
      if (!agent) return;
      setDraftAction((m) => ({ ...m, [uri]: "deleting" }));
      try {
        await agent.place.stream.vod.deleteDraft({ uri });
        setDrafts((prev) => prev.filter((d) => d.uri !== uri));
      } catch (err) {
        console.error("Failed to delete draft", err);
        setDraftAction((m) => ({ ...m, [uri]: "error" }));
      }
    },
    [agent],
  );

  // ── render ────────────────────────────────────────────────────────────────

  // Login gate: uploading needs an account. Wait for auth to resolve, then
  // prompt logged-out users with a login button that returns them here.
  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return (
      <View
        style={[
          zero.flex.values[1],
          zero.layout.flex.align.center,
          zero.layout.flex.justify.center,
          zero.px[6],
          zero.py[12],
        ]}
      >
        <View
          style={{
            maxWidth: 440,
            width: "100%",
            alignItems: "center",
            gap: 16,
          }}
        >
          <Text size="2xl" weight="semibold" style={{ textAlign: "center" }}>
            Log in to upload videos
          </Text>
          <Text color="muted" center>
            You need to be logged in to upload and manage your videos.
          </Text>
          <Button
            onPress={() =>
              openLoginModal({ name: route.name, params: route.params })
            }
          >
            <Text style={{ color: "#fff", fontWeight: "600" }}>Log in</Text>
          </Button>
        </View>
      </View>
    );
  }

  // Beta gate: hold the upload UI until we know the account's access status.
  // While we're still resolving it for a logged-in user, show a spinner; if
  // they're not granted, swap the whole page for the request-access flow.
  // Logged-out users keep the legacy behavior (status stays null) and are
  // prompted to log in only when they actually try to upload.
  if (agent?.did && betaStatus === null && betaLoading) {
    return <Loading />;
  }
  if (betaStatus === "none" || betaStatus === "requested") {
    return (
      <BetaAccessGate
        feature={BETA_FEATURE}
        status={betaStatus}
        onRequested={markRequested}
      />
    );
  }

  const renderTab = (
    mode: "upload" | "videos" | "livestreams" | "drafts",
    label: string,
    position: "left" | "middle" | "right",
  ) => {
    const active = managerMode === mode;
    const radiusOverride =
      position === "left"
        ? { borderTopRightRadius: 0, borderBottomRightRadius: 0 }
        : position === "right"
          ? { borderTopLeftRadius: 0, borderBottomLeftRadius: 0 }
          : { borderRadius: 0 };
    return (
      <Pressable
        onPress={() => setManagerMode(mode)}
        style={[
          zero.px[4],
          zero.py[2],
          zero.r.lg,
          radiusOverride,
          active
            ? { backgroundColor: theme.colors.primary }
            : {
                backgroundColor: "transparent",
                borderWidth: 1,
                borderColor: theme.colors.border,
                ...(position === "left" ? {} : { borderLeftWidth: 0 }),
              },
        ]}
      >
        <Text
          style={{
            color: active ? "#fff" : theme.colors.foreground,
            fontWeight: "600",
            fontSize: 14,
          }}
        >
          {label}
        </Text>
      </Pressable>
    );
  };

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        {/* Mode toggle */}
        <View
          style={{
            maxWidth: 960,
            width: "100%",
            flexDirection: "row",
            marginBottom: 16,
            gap: 0,
          }}
        >
          {renderTab("upload", "Upload", "left")}
          {renderTab("videos", "My Videos", "middle")}
          {renderTab("livestreams", "Livestreams", "middle")}
          {renderTab("drafts", "Drafts", "right")}
        </View>

        {/* Video list mode */}
        {managerMode === "videos" && (
          <View style={{ maxWidth: 960, width: "100%", gap: 12 }}>
            {userVideos.length === 0 && (
              <View style={[zero.p[6], { alignItems: "center" }]}>
                <Video size={48} color={theme.colors.mutedForeground} />
                <Text
                  color="muted"
                  size="sm"
                  style={{ marginTop: 12, textAlign: "center" }}
                >
                  No videos yet. Switch to the Upload tab to add your first
                  video.
                </Text>
              </View>
            )}
            {userVideos.map((video: any) => {
              const rec = video.record?.value || video.record || {};
              const thumb = rec.thumb;
              const thumbUrl = thumb
                ? `https://cdn.stream.place/thumb/${thumb.ref?.$link || thumb.cid || ""}`
                : undefined;
              return (
                <Pressable
                  key={video.uri}
                  onPress={() => handleSelectVideo(video)}
                  style={[
                    zero.p[3],
                    zero.r.md,
                    {
                      flexDirection: "row",
                      gap: 12,
                      borderWidth: 1,
                      borderColor: theme.colors.border,
                      backgroundColor: theme.colors.background,
                      alignItems: "center",
                    },
                  ]}
                >
                  {thumbUrl ? (
                    <Image
                      source={{ uri: thumbUrl }}
                      style={{
                        width: 80,
                        height: 45,
                        borderRadius: 4,
                      }}
                      contentFit="cover"
                    />
                  ) : (
                    <View
                      style={{
                        width: 80,
                        height: 45,
                        borderRadius: 4,
                        backgroundColor: theme.colors.muted,
                        alignItems: "center",
                        justifyContent: "center",
                      }}
                    >
                      <Video size={20} color={theme.colors.mutedForeground} />
                    </View>
                  )}
                  <View style={{ flex: 1, gap: 2 }}>
                    <Text
                      numberOfLines={1}
                      style={{ fontWeight: "600", fontSize: 14 }}
                    >
                      {rec.title || "Untitled"}
                    </Text>
                    <Text size="xs" color="muted">
                      {video.viewCounts?.count != null &&
                        `${video.viewCounts.count} views · `}
                      {rec.durationMs
                        ? `${Math.round(rec.durationMs / 1000)}s`
                        : ""}
                    </Text>
                  </View>
                </Pressable>
              );
            })}
          </View>
        )}

        {/* Livestreams list mode */}
        {managerMode === "livestreams" && (
          <View style={{ maxWidth: 960, width: "100%", gap: 12 }}>
            {livestreamsLoading && livestreams.length === 0 && (
              <View style={[zero.p[6], { alignItems: "center" }]}>
                <LoaderCircle size={32} color={theme.colors.mutedForeground} />
              </View>
            )}
            {!livestreamsLoading && livestreams.length === 0 && (
              <View style={[zero.p[6], { alignItems: "center" }]}>
                <Video size={48} color={theme.colors.mutedForeground} />
                <Text
                  color="muted"
                  size="sm"
                  style={{ marginTop: 12, textAlign: "center" }}
                >
                  No livestreams yet. When you go live, your past streams show
                  up here to finalize into VODs.
                </Text>
              </View>
            )}
            {livestreams.map((ls: any) => {
              const rec = ls.value || {};
              const ended = !!rec.endedAt;
              const vodUri = livestreamToVideo.get(ls.uri);
              const status = finalizing[ls.uri];
              return (
                <View
                  key={ls.uri}
                  style={[
                    zero.p[3],
                    zero.r.md,
                    {
                      flexDirection: "row",
                      gap: 12,
                      borderWidth: 1,
                      borderColor: theme.colors.border,
                      backgroundColor: theme.colors.background,
                      alignItems: "center",
                    },
                  ]}
                >
                  <View style={{ flex: 1, gap: 2 }}>
                    <Text
                      numberOfLines={1}
                      style={{ fontWeight: "600", fontSize: 14 }}
                    >
                      {rec.title || "Untitled livestream"}
                    </Text>
                    <Text size="xs" color="muted">
                      {rec.createdAt
                        ? new Date(rec.createdAt).toLocaleString()
                        : ""}
                      {ended ? " · Ended" : " · Live"}
                    </Text>
                  </View>
                  {vodUri ? (
                    <AQLink
                      to={{
                        screen: "Video",
                        params: {
                          user: agent?.did ?? "",
                          tid: vodUri.split("/").pop() ?? "",
                        },
                      }}
                    >
                      <View
                        style={{
                          flexDirection: "row",
                          alignItems: "center",
                          gap: 6,
                        }}
                      >
                        <CheckCircle2 size={16} color={theme.colors.primary} />
                        <Text size="sm" style={{ color: theme.colors.primary }}>
                          View VOD
                        </Text>
                      </View>
                    </AQLink>
                  ) : status === "finalizing" ? (
                    <View
                      style={{
                        flexDirection: "row",
                        alignItems: "center",
                        gap: 6,
                      }}
                    >
                      <LoaderCircle
                        size={16}
                        color={theme.colors.mutedForeground}
                      />
                      <Text size="xs" color="muted">
                        Finalizing…
                      </Text>
                    </View>
                  ) : status === "done" ? (
                    <Pressable
                      onPress={() => setManagerMode("drafts")}
                      style={{
                        flexDirection: "row",
                        alignItems: "center",
                        gap: 6,
                      }}
                    >
                      <CheckCircle2 size={16} color={theme.colors.success} />
                      <Text size="sm" style={{ color: theme.colors.success }}>
                        Find in Drafts →
                      </Text>
                    </Pressable>
                  ) : !ended ? (
                    <Tooltip content="This stream is still live. Finalize once it has ended.">
                      <Button size="sm" disabled onPress={() => {}}>
                        <Text style={{ color: "#fff" }}>Finalize</Text>
                      </Button>
                    </Tooltip>
                  ) : (
                    <Button
                      size="sm"
                      onPress={() => finalizeLivestreamRow(ls)}
                      style={{ width: "auto" }}
                    >
                      <Text style={{ color: "#fff" }}>
                        {status === "error" ? "Retry" : "Finalize"}
                      </Text>
                    </Button>
                  )}
                </View>
              );
            })}
          </View>
        )}

        {/* Drafts list mode */}
        {managerMode === "drafts" && (
          <View style={{ maxWidth: 960, width: "100%", gap: 12 }}>
            {draftsLoading && drafts.length === 0 && (
              <View style={[zero.p[6], { alignItems: "center" }]}>
                <LoaderCircle size={32} color={theme.colors.mutedForeground} />
              </View>
            )}
            {!draftsLoading && drafts.length === 0 && (
              <View style={[zero.p[6], { alignItems: "center" }]}>
                <Video size={48} color={theme.colors.mutedForeground} />
                <Text
                  color="muted"
                  size="sm"
                  style={{ marginTop: 12, textAlign: "center" }}
                >
                  No drafts. Uploads and finalized livestreams appear here as
                  drafts while they process — come back to add details and
                  publish.
                </Text>
              </View>
            )}
            {drafts.map((draft) => {
              const rec = draft.record as PlaceStreamVodDraftVideo.Main;
              const status = rec.status;
              const action = draftAction[draft.uri];
              const publishedUri = publishedFromDrafts[draft.uri];
              return (
                <View
                  key={draft.uri}
                  style={[
                    zero.p[3],
                    zero.r.md,
                    {
                      flexDirection: "row",
                      gap: 12,
                      borderWidth: 1,
                      borderColor: theme.colors.border,
                      backgroundColor: theme.colors.background,
                      alignItems: "center",
                    },
                  ]}
                >
                  <View style={{ flex: 1, gap: 2 }}>
                    <Text
                      numberOfLines={1}
                      style={{ fontWeight: "600", fontSize: 14 }}
                    >
                      {rec.title || "Untitled"}
                    </Text>
                    <Text size="xs" color="muted">
                      {rec.createdAt
                        ? new Date(rec.createdAt).toLocaleString()
                        : ""}
                      {" · "}
                      {status === "processing"
                        ? "Processing"
                        : status === "error"
                          ? `Error: ${rec.error || "unknown"}`
                          : "Ready"}
                    </Text>
                  </View>
                  {publishedUri ? (
                    <AQLink
                      to={{
                        screen: "Video",
                        params: {
                          user: agent?.did ?? "",
                          tid: publishedUri.split("/").pop() ?? "",
                        },
                      }}
                    >
                      <View
                        style={{
                          flexDirection: "row",
                          alignItems: "center",
                          gap: 6,
                        }}
                      >
                        <CheckCircle2 size={16} color={theme.colors.primary} />
                        <Text size="sm" style={{ color: theme.colors.primary }}>
                          View VOD
                        </Text>
                      </View>
                    </AQLink>
                  ) : action === "publishing" || action === "deleting" ? (
                    <View
                      style={{
                        flexDirection: "row",
                        alignItems: "center",
                        gap: 6,
                      }}
                    >
                      <LoaderCircle
                        size={16}
                        color={theme.colors.mutedForeground}
                      />
                      <Text size="xs" color="muted">
                        {action === "publishing" ? "Publishing…" : "Deleting…"}
                      </Text>
                    </View>
                  ) : status === "ready" ? (
                    <View style={{ flexDirection: "row", gap: 8 }}>
                      <Button
                        size="sm"
                        variant="secondary"
                        onPress={() => handleSelectDraft(draft)}
                      >
                        Edit
                      </Button>
                      <Button
                        size="sm"
                        onPress={() => handlePublishDraft(draft.uri)}
                      >
                        Publish
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        onPress={() => handleDeleteDraft(draft.uri)}
                      >
                        <X size={14} color="#fff" />
                      </Button>
                    </View>
                  ) : status === "error" ? (
                    <Button
                      size="sm"
                      variant="destructive"
                      onPress={() => handleDeleteDraft(draft.uri)}
                    >
                      Discard
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      variant="destructive"
                      onPress={() => handleDeleteDraft(draft.uri)}
                    >
                      <X size={14} color="#fff" />
                    </Button>
                  )}
                </View>
              );
            })}
          </View>
        )}

        {/* Upload/Edit mode */}
        {managerMode === "upload" && (
          <>
            {(editingVideoUri || editingDraftUri) && (
              <View
                style={{
                  maxWidth: 960,
                  width: "100%",
                  flexDirection: "row",
                  justifyContent: "space-between",
                  alignItems: "center",
                  marginBottom: 12,
                }}
              >
                <Text size="sm" color="muted">
                  {editingDraftUri ? "Editing draft" : "Editing video"}
                </Text>
                <View style={{ flexDirection: "row", gap: 8 }}>
                  {editingVideoUri && (
                    <Button
                      variant="secondary"
                      size="sm"
                      onPress={handleDeleteVideo}
                      disabled={deleting}
                    >
                      <Text style={{ color: theme.colors.destructive }}>
                        {deleting ? "Deleting…" : "Delete"}
                      </Text>
                    </Button>
                  )}
                  <Pressable
                    onPress={() => {
                      setEditingVideoUri(undefined);
                      setEditingDraftUri(undefined);
                      setPhase({ kind: "idle" });
                      setFile(null);
                      setTitle("");
                      setDescription("");
                      setActivity(undefined);
                      setTags([]);
                      setTagInput("");
                      setThumbnail(undefined);
                      setThumbnailUrl(undefined);
                      setWarnings(new Set());
                      setLicense(
                        "place.stream.metadata.contentRights#all-rights-reserved",
                      );
                    }}
                  >
                    <X size={18} color={theme.colors.mutedForeground} />
                  </Pressable>
                </View>
              </View>
            )}
            <View
              style={{
                maxWidth: 960,
                width: "100%",
                flexDirection: isWide ? "row" : "column",
                alignItems: "flex-start",
              }}
            >
              {/* ── left column: metadata ────────────────────────────────────── */}
              <View
                style={{
                  flex: isWide ? 3 : undefined,
                  width: isWide ? undefined : "100%",
                }}
              >
                <MenuContainer>
                  {/* title + description */}
                  <View style={{ gap: 6 }}>
                    <MenuLabel>Details</MenuLabel>
                    <MenuGroup>
                      <View style={[zero.px[3], zero.py[1], { gap: 4 }]}>
                        <Text size="sm" color="muted">
                          Title
                        </Text>
                        <TextInput
                          value={title}
                          onChangeText={setTitle}
                          maxLength={140}
                          placeholder="Give your video a title"
                          placeholderTextColor={theme.colors.mutedForeground}
                          style={inputStyle(theme)}
                        />
                      </View>
                      <MenuSeparator />
                      <View style={[zero.px[3], zero.py[1], { gap: 4 }]}>
                        <Text size="sm" color="muted">
                          Description
                        </Text>
                        <TextInput
                          value={description}
                          onChangeText={setDescription}
                          maxLength={5000}
                          multiline
                          numberOfLines={4}
                          placeholder="Describe your video…"
                          placeholderTextColor={theme.colors.mutedForeground}
                          textAlignVertical="top"
                          style={[inputStyle(theme), { minHeight: 80 }]}
                        />
                      </View>
                    </MenuGroup>
                  </View>

                  {/* activity */}
                  <View style={{ gap: 6 }}>
                    <MenuLabel>Activity</MenuLabel>
                    <MenuGroup>
                      <View style={[zero.px[3], zero.py[2]]}>
                        <ActivityPicker
                          value={activity}
                          onChange={setActivity}
                        />
                      </View>
                    </MenuGroup>
                  </View>

                  {/* tags */}
                  <View style={{ gap: 6 }}>
                    <MenuLabel>Tags</MenuLabel>
                    <MenuGroup>
                      <View style={[zero.px[3], zero.py[2], { gap: 8 }]}>
                        {tags.length > 0 && (
                          <View
                            style={{
                              flexDirection: "row",
                              flexWrap: "wrap",
                              gap: 6,
                            }}
                          >
                            {tags.map((tag) => (
                              <Pressable
                                key={tag}
                                onPress={() =>
                                  setTags(tags.filter((t) => t !== tag))
                                }
                                style={{
                                  flexDirection: "row",
                                  alignItems: "center",
                                  backgroundColor: theme.colors.primary + "22",
                                  borderRadius: 12,
                                  paddingHorizontal: 10,
                                  paddingVertical: 3,
                                }}
                              >
                                <Text
                                  size="sm"
                                  style={{ color: theme.colors.primary }}
                                >
                                  {tag}
                                </Text>
                                <Text
                                  size="sm"
                                  style={{
                                    color: theme.colors.primary,
                                    marginLeft: 4,
                                  }}
                                >
                                  ×
                                </Text>
                              </Pressable>
                            ))}
                          </View>
                        )}
                        {tags.length < 10 && (
                          <TextInput
                            value={tagInput}
                            onChangeText={(v) =>
                              setTagInput(v.replace(/[^a-zA-Z0-9:]/g, ""))
                            }
                            placeholder="Add tag, press Enter"
                            placeholderTextColor={theme.colors.mutedForeground}
                            returnKeyType="done"
                            onSubmitEditing={() => {
                              const t = tagInput.trim();
                              if (t && !tags.includes(t)) setTags([...tags, t]);
                              setTagInput("");
                            }}
                            style={inputStyle(theme)}
                          />
                        )}
                      </View>
                    </MenuGroup>
                  </View>

                  {/* content warnings */}
                  <View style={{ gap: 6 }}>
                    <MenuLabel>Content Warnings</MenuLabel>
                    <MenuGroup>
                      <View style={[zero.px[3], zero.py[2], { gap: 8 }]}>
                        {CONTENT_WARNINGS.map((cw) => (
                          <Tooltip
                            key={cw.value}
                            content={cw.description}
                            position="top"
                          >
                            <Checkbox
                              checked={warnings.has(cw.value)}
                              onCheckedChange={() => toggleWarning(cw.value)}
                              label={cw.label}
                              size="sm"
                            />
                          </Tooltip>
                        ))}
                      </View>
                    </MenuGroup>
                  </View>

                  {/* license */}
                  <View style={{ gap: 6 }}>
                    <MenuLabel>License</MenuLabel>
                    <MenuGroup>
                      <View style={[zero.px[3], zero.py[2]]}>
                        <Select
                          value={license}
                          onValueChange={setLicense}
                          placeholder="All rights reserved"
                          items={LICENSE_OPTIONS.map((opt) => ({
                            label: opt.label,
                            value: opt.value,
                            description: opt.description,
                          }))}
                          style={[
                            zero.p[3],
                            zero.r.md,
                            { borderColor: theme.colors.border },
                            zero.borders.width.thin,
                            { backgroundColor: theme.colors.background },
                          ]}
                        />
                      </View>
                    </MenuGroup>
                  </View>
                </MenuContainer>
              </View>

              {/* ── right column: file + status + actions ────────────────────── */}
              {!editingVideoUri && !editingDraftUri && (
                <View
                  style={{
                    flex: isWide ? 2 : undefined,
                    width: isWide ? undefined : "100%",
                  }}
                >
                  <MenuContainer>
                    <View
                      style={{
                        gap: 6,
                      }}
                    >
                      {/* file picker */}
                      <MenuLabel>File</MenuLabel>
                      <MenuGroup>
                        <Pressable
                          onPress={pickFile}
                          disabled={isUploading}
                          style={[
                            zero.px[3],
                            zero.py[3],
                            zero.r.md,
                            zero.borders.width.medium,
                            zero.borders.style.dashed,
                            zt.border.border,
                            {
                              opacity: isUploading ? 0.5 : 1,
                              height: 120,
                              alignItems: "center",
                              justifyContent: "center",
                              gap: 4,
                            },
                          ]}
                        >
                          <Video
                            size={32}
                            color={theme.colors.mutedForeground}
                          />
                          <Text weight="medium">
                            {file ? file.name : "Choose a video file"}
                          </Text>
                          {file && (
                            <Text size="sm" color="muted">
                              {file.type || "unknown"} — {humanBytes(file.size)}
                            </Text>
                          )}
                        </Pressable>
                      </MenuGroup>

                      {/* thumbnail */}
                      <MenuGroup>
                        <MenuLabel>Thumbnail</MenuLabel>
                        <View style={[zero.px[3], zero.py[2], { gap: 8 }]}>
                          {thumbnailUrl ? (
                            <View style={{ position: "relative" }}>
                              <Image
                                source={{ uri: thumbnailUrl }}
                                style={{
                                  width: "100%",
                                  height: 200,
                                  borderRadius: 8,
                                }}
                                contentFit="cover"
                              />
                              <Pressable
                                onPress={handleThumbnailRemove}
                                style={{
                                  position: "absolute",
                                  top: 8,
                                  right: 8,
                                  backgroundColor: "rgba(0, 0, 0, 0.7)",
                                  borderRadius: 12,
                                  width: 24,
                                  height: 24,
                                  alignItems: "center",
                                  justifyContent: "center",
                                }}
                              >
                                <X size={16} color="white" />
                              </Pressable>
                            </View>
                          ) : (
                            <Pressable
                              onPress={handleThumbnailSelect}
                              style={[
                                zero.borders.width.medium,
                                { borderColor: theme.colors.border },
                                zero.r.md,
                                {
                                  height: 120,
                                  borderStyle: "dashed",
                                  alignItems: "center",
                                  justifyContent: "center",
                                  gap: 4,
                                  backgroundColor: theme.colors.background,
                                },
                              ]}
                            >
                              <ImagePlus
                                size={32}
                                color={theme.colors.mutedForeground}
                              />
                              <Text size="sm" color="muted">
                                Add thumbnail image
                              </Text>
                              <Text size="xs" color="muted">
                                Optional · JPG, PNG up to 975KB
                              </Text>
                            </Pressable>
                          )}
                          {thumbnail && thumbnail.size > 975000 && (
                            <Admonition variant="warning" size="sm">
                              <Text size="sm">
                                Heads up: this image is larger than 975KB.
                                Upload might fail.
                              </Text>
                            </Admonition>
                          )}
                          <input
                            type="file"
                            accept="image/*"
                            ref={thumbnailInputRef}
                            onChange={handleThumbnailChange}
                            style={{ display: "none" }}
                          />
                        </View>
                      </MenuGroup>

                      {/* status */}
                      {phase.kind !== "idle" && (
                        <MenuGroup>
                          <MenuLabel>Status</MenuLabel>
                          <View style={[zero.px[3], zero.py[2], { gap: 12 }]}>
                            {phase.kind === "creating" && (
                              <View style={{ gap: 8 }}>
                                <View
                                  style={{
                                    flexDirection: "row",
                                    alignItems: "center",
                                    gap: 10,
                                  }}
                                >
                                  <LoaderCircle
                                    size={18}
                                    color={theme.colors.mutedForeground}
                                  />
                                  <Text size="sm" color="muted">
                                    Preparing upload…
                                  </Text>
                                </View>
                                <Animated.View
                                  style={{
                                    height: 6,
                                    borderRadius: 3,
                                    backgroundColor: theme.colors.muted,
                                    overflow: "hidden",
                                    width: "100%",
                                  }}
                                >
                                  <Animated.View
                                    style={{
                                      height: 6,
                                      borderRadius: 3,
                                      backgroundColor: theme.colors.primary,
                                      width: processingAnim.interpolate({
                                        inputRange: [0, 1],
                                        outputRange: ["0%", "100%"],
                                      }),
                                      opacity: processingAnim.interpolate({
                                        inputRange: [0, 0.5, 1],
                                        outputRange: [0.4, 1, 0.4],
                                      }),
                                    }}
                                  />
                                </Animated.View>
                              </View>
                            )}
                            {phase.kind === "uploading" && (
                              <View style={{ gap: 8 }}>
                                <View
                                  style={{
                                    flexDirection: "row",
                                    alignItems: "center",
                                    gap: 10,
                                  }}
                                >
                                  <ArrowUp
                                    size={18}
                                    color={theme.colors.primary}
                                  />
                                  <Text size="sm">
                                    {phase.pct.toFixed(1)}% —{" "}
                                    {humanBytes(phase.bytesSent)} /{" "}
                                    {humanBytes(phase.bytesTotal)}
                                  </Text>
                                </View>
                                <View
                                  style={{
                                    height: 6,
                                    borderRadius: 3,
                                    backgroundColor: theme.colors.muted,
                                    overflow: "hidden",
                                    width: "100%",
                                  }}
                                >
                                  <View
                                    style={{
                                      height: 6,
                                      width: `${phase.pct}%`,
                                      backgroundColor: theme.colors.primary,
                                      borderRadius: 3,
                                    }}
                                  />
                                </View>
                              </View>
                            )}
                            {(phase.kind === "processing" ||
                              phase.kind === "publishing") && (
                              <View style={{ gap: 8 }}>
                                <View
                                  style={{
                                    flexDirection: "row",
                                    alignItems: "center",
                                    gap: 10,
                                  }}
                                >
                                  <LoaderCircle
                                    size={18}
                                    color={theme.colors.mutedForeground}
                                  />
                                  <Text size="sm" color="muted">
                                    {phase.kind === "processing"
                                      ? phase.serverStatus === "processing"
                                        ? `Processing video${phase.progress != null ? ` (${phase.progress}%)` : "…"}`
                                        : "Waiting to process…"
                                      : "Publishing…"}
                                  </Text>
                                </View>
                                {phase.kind === "processing" &&
                                phase.progress != null ? (
                                  <View
                                    style={{
                                      height: 6,
                                      borderRadius: 3,
                                      backgroundColor: theme.colors.muted,
                                      overflow: "hidden",
                                      width: "100%",
                                    }}
                                  >
                                    <View
                                      style={{
                                        height: 6,
                                        width: `${phase.progress}%`,
                                        backgroundColor: theme.colors.primary,
                                        borderRadius: 3,
                                      }}
                                    />
                                  </View>
                                ) : (
                                  <Animated.View
                                    style={{
                                      height: 6,
                                      borderRadius: 3,
                                      backgroundColor: theme.colors.muted,
                                      overflow: "hidden",
                                      width: "100%",
                                    }}
                                  >
                                    <Animated.View
                                      style={{
                                        height: 6,
                                        borderRadius: 3,
                                        backgroundColor: theme.colors.primary,
                                        width: processingAnim.interpolate({
                                          inputRange: [0, 1],
                                          outputRange: ["0%", "100%"],
                                        }),
                                        opacity: processingAnim.interpolate({
                                          inputRange: [0, 0.5, 1],
                                          outputRange: [0.4, 1, 0.4],
                                        }),
                                      }}
                                    />
                                  </Animated.View>
                                )}
                              </View>
                            )}
                            {phase.kind === "ready" && (
                              <View
                                style={{
                                  flexDirection: "row",
                                  alignItems: "center",
                                  gap: 10,
                                }}
                              >
                                <CheckCircle2
                                  size={18}
                                  color={theme.colors.success}
                                />
                                <Text
                                  size="sm"
                                  style={{ color: theme.colors.success }}
                                >
                                  Ready to publish
                                </Text>
                              </View>
                            )}
                            {phase.kind === "done" && (
                              <View style={{ gap: 8 }}>
                                <View
                                  style={{
                                    flexDirection: "row",
                                    alignItems: "center",
                                    gap: 10,
                                  }}
                                >
                                  <CheckCircle2
                                    size={18}
                                    color={theme.colors.success}
                                  />
                                  <Text
                                    size="sm"
                                    style={{ color: theme.colors.success }}
                                  >
                                    Upload complete
                                  </Text>
                                </View>
                                <Pressable
                                  onPress={() => setManagerMode("drafts")}
                                  style={{
                                    flexDirection: "row",
                                    alignItems: "center",
                                    gap: 6,
                                  }}
                                >
                                  <Text
                                    size="sm"
                                    style={{ color: theme.colors.primary }}
                                  >
                                    Find your draft in the Drafts tab to add
                                    details and publish →
                                  </Text>
                                </Pressable>
                              </View>
                            )}
                            {phase.kind === "error" && (
                              <View
                                style={{
                                  flexDirection: "row",
                                  alignItems: "center",
                                  gap: 10,
                                }}
                              >
                                <AlertCircle
                                  size={18}
                                  color={theme.colors.destructive}
                                />
                                <Text size="sm" color="destructive">
                                  {phase.message}
                                </Text>
                              </View>
                            )}
                          </View>
                        </MenuGroup>
                      )}

                      {/* actions */}
                      <MenuGroup>
                        <View style={[zero.px[3], zero.py[2], { gap: 8 }]}>
                          {editingDraftUri ? (
                            <Button
                              onPress={handleUpdateDraft}
                              disabled={!title.trim() || updating}
                            >
                              {updating ? "Saving…" : "Save Draft"}
                            </Button>
                          ) : editingVideoUri ? (
                            <Button
                              onPress={handleUpdateVideo}
                              disabled={!title.trim() || updating}
                            >
                              {updating ? "Updating…" : "Update Video"}
                            </Button>
                          ) : (
                            <>
                              {canUpload && (
                                <Button onPress={startUpload}>Upload</Button>
                              )}
                              {!file && phase.kind === "idle" && (
                                <Button onPress={pickFile} variant="secondary">
                                  Choose file
                                </Button>
                              )}
                              {isUploading && phase.kind !== "processing" && (
                                <Button
                                  variant="destructive"
                                  onPress={cancelUpload}
                                >
                                  Cancel
                                </Button>
                              )}
                              {phase.kind === "done" && (
                                <Button
                                  onPress={() => {
                                    setFile(null);
                                    setPhase({ kind: "idle" });
                                    setTitle("");
                                    setDescription("");
                                    setActivity(undefined);
                                    setTags([]);
                                    setTagInput("");
                                    setThumbnail(undefined);
                                    setThumbnailUrl(undefined);
                                    setWarnings(new Set());
                                    setLicense(
                                      "place.stream.metadata.contentRights#all-rights-reserved",
                                    );
                                  }}
                                  variant="secondary"
                                >
                                  Upload another
                                </Button>
                              )}
                            </>
                          )}
                        </View>
                      </MenuGroup>
                    </View>
                  </MenuContainer>
                </View>
              )}
            </View>
          </>
        )}
      </View>

      <input
        type="file"
        accept="video/*"
        ref={fileInputRef}
        onChange={handleFileChange}
        style={{ display: "none" }}
      />
    </ScrollView>
  );
}

// ── style helpers ─────────────────────────────────────────────────────────────

function inputStyle(theme: any) {
  return {
    borderWidth: 1,
    borderColor: theme.colors.border,
    borderRadius: 8,
    padding: 8,
    color: theme.colors.foreground,
    backgroundColor: theme.colors.background,
  };
}
