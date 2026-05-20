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
import { useCallback, useEffect, useRef, useState } from "react";
import {
  Animated,
  Pressable,
  ScrollView,
  TextInput,
  useWindowDimensions,
} from "react-native";
import type { PlaceStreamLivestream, PlaceStreamVideo } from "streamplace";
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
  | { kind: "ready"; uploadId: string; tracks: TrackRef[]; durationMs: number }
  | { kind: "publishing" }
  | { kind: "done"; videoUri: string }
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

const POLL_INTERVAL_MS = 3000;

// ── screen ───────────────────────────────────────────────────────────────────

export default function UploadScreen() {
  const agent = usePDSAgent();
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

  // ── polling ──────────────────────────────────────────────────────────────

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

  // ── publish ──────────────────────────────────────────────────────────────

  const publish = useCallback(async () => {
    if (phase.kind !== "ready" || !agent || !agent.did) return;
    setPhase({ kind: "publishing" });
    try {
      const { tracks, durationMs } = phase;

      const record: PlaceStreamVideo.Record = {
        $type: "place.stream.video",
        title: title.trim() || file?.name || "Untitled",
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
      if (activity) record.activity = activity;
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

      // Publish server-side: the server owns the authoritative source
      // tracks + durationMs (overriding whatever we sent) and backfills a
      // generated thumbnail when we didn't supply one.
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
    activity,
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

  // ── VOD manager ────────────────────────────────────────────────────────────

  const [managerMode, setManagerMode] = useState<"upload" | "videos">("upload");
  const [userVideos, setUserVideos] = useState<any[]>([]);
  const [editingVideoUri, setEditingVideoUri] = useState<string | undefined>(
    undefined,
  );
  const [updating, setUpdating] = useState(false);
  const [deleting, setDeleting] = useState(false);

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

  useEffect(() => {
    if (managerMode === "videos") {
      fetchVideos();
    }
  }, [managerMode, fetchVideos]);

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
      if (description.trim()) record.description = description.trim();
      if (activity) record.activity = activity;
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
        };
      }
      if (
        license &&
        license !== "place.stream.metadata.contentRights#all-rights-reserved"
      ) {
        const licenseKey = license.split("#")[1];
        record.contentRights = {
          $type: "place.stream.metadata.contentRights",
          license: { $type: license },
          [licenseKey]: { $type: license },
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

  // ── render ────────────────────────────────────────────────────────────────

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
          <Pressable
            onPress={() => setManagerMode("upload")}
            style={[
              zero.px[4],
              zero.py[2],
              zero.r.lg,
              {
                borderBottomRightRadius: 0,
                borderTopRightRadius: 0,
              },
              managerMode === "upload"
                ? { backgroundColor: theme.colors.primary }
                : {
                    backgroundColor: "transparent",
                    borderWidth: 1,
                    borderColor: theme.colors.border,
                  },
            ]}
          >
            <Text
              style={{
                color:
                  managerMode === "upload" ? "#fff" : theme.colors.foreground,
                fontWeight: "600",
                fontSize: 14,
              }}
            >
              Upload
            </Text>
          </Pressable>
          <Pressable
            onPress={() => setManagerMode("videos")}
            style={[
              zero.px[4],
              zero.py[2],
              zero.r.lg,
              {
                borderBottomLeftRadius: 0,
                borderTopLeftRadius: 0,
              },
              managerMode === "videos"
                ? { backgroundColor: theme.colors.primary }
                : {
                    backgroundColor: "transparent",
                    borderWidth: 1,
                    borderColor: theme.colors.border,
                    borderLeftWidth: 0,
                  },
            ]}
          >
            <Text
              style={{
                color:
                  managerMode === "videos" ? "#fff" : theme.colors.foreground,
                fontWeight: "600",
                fontSize: 14,
              }}
            >
              My Videos
            </Text>
          </Pressable>
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

        {/* Upload/Edit mode */}
        {managerMode === "upload" && (
          <>
            {editingVideoUri && (
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
                  Editing video
                </Text>
                <View style={{ flexDirection: "row", gap: 8 }}>
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
                  <Pressable
                    onPress={() => {
                      setEditingVideoUri(undefined);
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
              {!editingVideoUri && (
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
                                  Published
                                </Text>
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
                          {editingVideoUri ? (
                            <>
                              <Button
                                onPress={handleUpdateVideo}
                                disabled={!title.trim() || updating}
                              >
                                {updating ? "Updating…" : "Update Video"}
                              </Button>
                            </>
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
                              {(phase.kind === "ready" || isPublishing) && (
                                <Button
                                  onPress={publish}
                                  disabled={!canPublish || isPublishing}
                                >
                                  {isPublishing ? "Publishing…" : "Publish"}
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
