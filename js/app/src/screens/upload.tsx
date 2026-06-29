import { useNavigation, useRoute } from "@react-navigation/native";
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

// Editable metadata fields shared by the draft editor and the published-video
// editor. Extracted so both flows render the same form.
type MetadataState = {
  title: string;
  description: string;
  activity: PlaceStreamLivestream.Record["activity"];
  tags: string[];
  tagInput: string;
  thumbnail: Blob | undefined;
  thumbnailUrl: string | undefined;
  warnings: Set<string>;
  license: string;
};

const DEFAULT_METADATA: MetadataState = {
  title: "",
  description: "",
  activity: undefined,
  tags: [],
  tagInput: "",
  thumbnail: undefined,
  thumbnailUrl: undefined,
  warnings: new Set<string>(),
  license: "place.stream.metadata.contentRights#all-rights-reserved",
};

const BETA_FEATURE = "vod";

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

// Extract the rkey (tid) from an at:// or ats:// URI. The tid is always the
// last path segment for both draft URIs and published-video URIs.
function tidFromUri(uri: string): string {
  return uri.split("/").pop() ?? "";
}

// Build the ats:// draft URI for the authenticated user + tid.
function draftUri(did: string, tid: string): string {
  return `ats://${did}/place.stream.vod.drafts/self/${did}/place.stream.vod.draftVideo/${tid}`;
}

// Build the at:// published-video URI for the authenticated user + tid.
function videoUri(did: string, tid: string): string {
  return `at://${did}/place.stream.video/${tid}`;
}

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

// ── login / beta gate (shared by all upload routes) ──────────────────────────

function useUploadGate() {
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

  if (!isReady) return { state: "loading" as const };
  if (!userProfile) {
    return {
      state: "login" as const,
      openLoginModal,
      route,
    };
  }
  if (agent?.did && betaStatus === null && betaLoading) {
    return { state: "loading" as const };
  }
  if (betaStatus === "none" || betaStatus === "requested") {
    return {
      state: "beta" as const,
      betaStatus,
      markRequested,
    };
  }
  return { state: "ok" as const, agent };
}

function UploadGate({ children }: { children: React.ReactNode }) {
  const gate = useUploadGate();
  const { theme } = useTheme();
  if (gate.state === "loading") return <Loading />;
  if (gate.state === "login") {
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
              gate.openLoginModal({
                name: gate.route.name,
                params: gate.route.params,
              })
            }
          >
            <Text style={{ color: "#fff", fontWeight: "600" }}>Log in</Text>
          </Button>
        </View>
      </View>
    );
  }
  if (gate.state === "beta") {
    return (
      <BetaAccessGate
        feature={BETA_FEATURE}
        status={gate.betaStatus}
        onRequested={gate.markRequested}
      />
    );
  }
  return <>{children}</>;
}

// ── tab nav row ──────────────────────────────────────────────────────────────

// The sub-nav row that used to switch managerMode. Now each entry is a real
// navigation.navigate() call. Renders the same segmented-control style.
function UploadTabNav() {
  const navigation = useNavigation();
  const { theme } = useTheme();
  const route = useRoute();

  type TabDef = {
    label: string;
    screen: "Upload" | "UploadVideos" | "UploadLivestreams" | "UploadDrafts";
    position: "left" | "middle" | "right";
  };
  const tabs: TabDef[] = [
    { label: "Upload", screen: "Upload", position: "left" },
    { label: "My Videos", screen: "UploadVideos", position: "middle" },
    { label: "Livestreams", screen: "UploadLivestreams", position: "middle" },
    { label: "Drafts", screen: "UploadDrafts", position: "right" },
  ];

  return (
    <View
      style={{
        maxWidth: 960,
        width: "100%",
        flexDirection: "row",
        marginBottom: 16,
        gap: 0,
      }}
    >
      {tabs.map((tab) => {
        const active = route.name === tab.screen;
        const radiusOverride =
          tab.position === "left"
            ? { borderTopRightRadius: 0, borderBottomRightRadius: 0 }
            : tab.position === "right"
              ? { borderTopLeftRadius: 0, borderBottomLeftRadius: 0 }
              : { borderRadius: 0 };
        return (
          <Pressable
            key={tab.screen}
            onPress={() => navigation.navigate(tab.screen as any)}
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
                    ...(tab.position === "left" ? {} : { borderLeftWidth: 0 }),
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
              {tab.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

// ── shared metadata editor ───────────────────────────────────────────────────

// The metadata form (title/description/tags/activity/content-warnings/license/
// thumbnail). Shared by the draft editor and the published-video editor.
function MetadataEditor({
  meta,
  setMeta,
}: {
  meta: MetadataState;
  setMeta: (
    updater: (prev: MetadataState) => MetadataState | Partial<MetadataState>,
  ) => void;
}) {
  const { theme } = useTheme();
  const thumbnailInputRef = useRef<HTMLInputElement | null>(null);

  const toggleWarning = useCallback(
    (value: string) => {
      setMeta((prev) => {
        const next = new Set(prev.warnings);
        next.has(value) ? next.delete(value) : next.add(value);
        return { warnings: next };
      });
    },
    [setMeta],
  );

  const handleThumbnailSelect = useCallback(() => {
    thumbnailInputRef.current?.click();
  }, []);

  const handleThumbnailChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0] ?? null;
      if (f) {
        const blob = new Blob([f], { type: f.type });
        const url = URL.createObjectURL(blob);
        setMeta((prev) => ({ ...prev, thumbnail: blob, thumbnailUrl: url }));
      }
      e.target.value = "";
    },
    [setMeta],
  );

  const handleThumbnailRemove = useCallback(() => {
    setMeta((prev) => ({
      ...prev,
      thumbnail: undefined,
      thumbnailUrl: undefined,
    }));
  }, [setMeta]);

  // clean up old blob URLs when thumbnail changes or unmounts
  useEffect(() => {
    return () => {
      if (meta.thumbnailUrl) URL.revokeObjectURL(meta.thumbnailUrl);
    };
  }, [meta.thumbnailUrl]);

  const setField = useCallback(
    <K extends keyof MetadataState>(key: K, value: MetadataState[K]) => {
      setMeta((prev) => ({ ...prev, [key]: value }));
    },
    [setMeta],
  );

  return (
    <View style={{ flex: 3, width: "100%", gap: 12 }}>
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
                value={meta.title}
                onChangeText={(v) => setField("title", v)}
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
                value={meta.description}
                onChangeText={(v) => setField("description", v)}
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
                value={meta.activity}
                onChange={(v) => setField("activity", v)}
              />
            </View>
          </MenuGroup>
        </View>

        {/* tags */}
        <View style={{ gap: 6 }}>
          <MenuLabel>Tags</MenuLabel>
          <MenuGroup>
            <View style={[zero.px[3], zero.py[2], { gap: 8 }]}>
              {meta.tags.length > 0 && (
                <View
                  style={{
                    flexDirection: "row",
                    flexWrap: "wrap",
                    gap: 6,
                  }}
                >
                  {meta.tags.map((tag) => (
                    <Pressable
                      key={tag}
                      onPress={() =>
                        setMeta((prev) => ({
                          ...prev,
                          tags: prev.tags.filter((t) => t !== tag),
                        }))
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
                      <Text size="sm" style={{ color: theme.colors.primary }}>
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
              {meta.tags.length < 10 && (
                <TextInput
                  value={meta.tagInput}
                  onChangeText={(v) =>
                    setField("tagInput", v.replace(/[^a-zA-Z0-9:]/g, ""))
                  }
                  placeholder="Add tag, press Enter"
                  placeholderTextColor={theme.colors.mutedForeground}
                  returnKeyType="done"
                  onSubmitEditing={() => {
                    const t = meta.tagInput.trim();
                    if (t && !meta.tags.includes(t))
                      setMeta((prev) => ({
                        ...prev,
                        tags: [...prev.tags, t],
                        tagInput: "",
                      }));
                    else setMeta((prev) => ({ ...prev, tagInput: "" }));
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
                <Tooltip key={cw.value} content={cw.description} position="top">
                  <Checkbox
                    checked={meta.warnings.has(cw.value)}
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
                value={meta.license}
                onValueChange={(v) => setField("license", v)}
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

        {/* thumbnail */}
        <View style={{ gap: 6 }}>
          <MenuLabel>Thumbnail</MenuLabel>
          <MenuGroup>
            <View style={[zero.px[3], zero.py[2], { gap: 8 }]}>
              {meta.thumbnailUrl ? (
                <View style={{ position: "relative" }}>
                  <Image
                    source={{ uri: meta.thumbnailUrl }}
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
                  <ImagePlus size={32} color={theme.colors.mutedForeground} />
                  <Text size="sm" color="muted">
                    Add thumbnail image
                  </Text>
                  <Text size="xs" color="muted">
                    Optional · JPG, PNG up to 975KB
                  </Text>
                </Pressable>
              )}
              {meta.thumbnail && meta.thumbnail.size > 975000 && (
                <Admonition variant="warning" size="sm">
                  <Text size="sm">
                    Heads up: this image is larger than 975KB. Upload might
                    fail.
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
        </View>
      </MenuContainer>
    </View>
  );
}

// ── 1. /upload — bare upload box ─────────────────────────────────────────────

export default function UploadScreen() {
  const gate = useUploadGate();
  const { theme, zero: zt } = useTheme();
  const { width } = useWindowDimensions();
  const navigation = useNavigation();

  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const uploadRef = useRef<tus.Upload | null>(null);
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const processingAnim = useRef(new Animated.Value(0)).current;
  const [file, setFile] = useState<File | null>(null);
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

  // cleanup on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearTimeout(pollRef.current);
      uploadRef.current?.abort();
    };
  }, []);

  const pickFile = useCallback(() => fileInputRef.current?.click(), []);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0] ?? null;
      setFile(f);
      setPhase({ kind: "idle" });
      e.target.value = "";
    },
    [],
  );

  const agent = gate.state === "ok" ? gate.agent : null;

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
      // Create the empty draft first — it's the durable anchor for this
      // upload. The user lands on the draft editor immediately and can edit
      // metadata while the upload runs; a failed upload leaves the draft
      // intact so they can re-upload against the same draft.
      const draftRes = await agent.place.stream.vod.createDraft({});
      if (!draftRes.success) throw new Error("createDraft failed");
      const draftUri = draftRes.data.uri;
      const tid = tidFromUri(draftUri);

      const res = await agent.place.stream.media.createUpload({
        size: file.size,
        mimeType,
        filename: file.name,
        draftUri,
      });
      if (!res.success) throw new Error("createUpload failed");
      const { uploadUrl, uploadToken } = res.data;

      // Navigate to the draft editor now — the upload continues in the
      // background and fills this draft when it finishes processing.
      navigation.navigate("UploadVideo" as any, { tid });

      // The TUS upload runs to completion here; the draft (already navigated
      // to) will flip to 'ready' server-side when processing finishes.
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
            // Progress isn't shown on the bare upload screen anymore (we've
            // navigated away to the editor); the editor surfaces the draft's
            // processing status instead.
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
      // No phase change here: we've already navigated to the draft editor,
      // which polls/reloads the draft and reflects the 'ready' state when
      // processing completes.
    } catch (err) {
      uploadRef.current = null;
      setPhase({
        kind: "error",
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }, [agent, file, navigation]);

  const cancelUpload = useCallback(() => {
    if (pollRef.current) clearTimeout(pollRef.current);
    uploadRef.current?.abort();
    uploadRef.current = null;
    setPhase({ kind: "idle" });
  }, []);

  const isUploading =
    phase.kind === "creating" ||
    phase.kind === "uploading" ||
    phase.kind === "processing";
  const canUpload = !!file && (phase.kind === "idle" || phase.kind === "error");

  if (gate.state !== "ok") {
    return <UploadGate>{null}</UploadGate>;
  }

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <UploadTabNav />

        <View style={{ maxWidth: 960, width: "100%" }}>
          <MenuContainer>
            <View style={{ gap: 6 }}>
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
                  <Video size={32} color={theme.colors.mutedForeground} />
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
                          <ArrowUp size={18} color={theme.colors.primary} />
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
                        <CheckCircle2 size={18} color={theme.colors.success} />
                        <Text size="sm" style={{ color: theme.colors.success }}>
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
                          onPress={() =>
                            navigation.navigate("UploadDrafts" as any)
                          }
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
                            Find your draft in the Drafts tab to add details and
                            publish →
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
                  {canUpload && <Button onPress={startUpload}>Upload</Button>}
                  {!file && phase.kind === "idle" && (
                    <Button onPress={pickFile} variant="secondary">
                      Choose file
                    </Button>
                  )}
                  {isUploading && phase.kind !== "processing" && (
                    <Button variant="destructive" onPress={cancelUpload}>
                      Cancel
                    </Button>
                  )}
                  {phase.kind === "done" && (
                    <Button
                      onPress={() => {
                        setFile(null);
                        setPhase({ kind: "idle" });
                      }}
                      variant="secondary"
                    >
                      Upload another
                    </Button>
                  )}
                </View>
              </MenuGroup>
            </View>
          </MenuContainer>
        </View>
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

// ── 2. /upload/video/:tid — unified draft/published editor ──────────────────

export function UploadVideoScreen({ route }: { route: any }) {
  const gate = useUploadGate();
  const { theme } = useTheme();
  const navigation = useNavigation();
  const { width } = useWindowDimensions();
  const isWide = width > 800;
  const { tid } = route.params as { tid: string };

  const [loading, setLoading] = useState(true);
  // "draft" | "video" | null until loaded
  const [mode, setMode] = useState<"draft" | "video" | null>(null);
  const [draftUri_, setDraftUri] = useState<string | undefined>(undefined);
  const [videoUri_, setVideoUri] = useState<string | undefined>(undefined);
  const [draftStatus, setDraftStatus] = useState<
    PlaceStreamVodDraftVideo.Main["status"] | undefined
  >(undefined);
  const [meta, setMetaState] = useState<MetadataState>(DEFAULT_METADATA);
  const [updating, setUpdating] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);

  const setMeta = useCallback(
    (
      updater: (prev: MetadataState) => MetadataState | Partial<MetadataState>,
    ) => {
      setMetaState((prev) => {
        const next = updater(prev);
        return { ...prev, ...(next as Partial<MetadataState>) };
      });
    },
    [],
  );

  const agent = gate.state === "ok" ? gate.agent : null;

  // Load the record: try draft first (getDraft), fall back to published video
  // (getRecord). Drafts and published videos share the same tid.
  useEffect(() => {
    if (!agent || !agent.did) return;
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    (async () => {
      const did = agent.did!;
      const duri = draftUri(did, tid);
      try {
        const res = await agent.place.stream.vod.getDraft({ uri: duri });
        if (cancelled) return;
        const rec = res.data.draft.record as PlaceStreamVodDraftVideo.Main;
        setMode("draft");
        setDraftUri(duri);
        setVideoUri(undefined);
        setDraftStatus(rec.status);
        setMetaState({
          title: rec.title || "",
          description: rec.description || "",
          activity: rec.activity || undefined,
          tags: rec.tags || [],
          tagInput: "",
          thumbnail: undefined,
          thumbnailUrl: rec.thumb
            ? `https://cdn.stream.place/thumb/${(rec.thumb.ref as any)?.$link || (rec.thumb as any).cid || ""}`
            : undefined,
          warnings: new Set(rec.contentWarnings?.warnings || []),
          license:
            (rec.contentRights as any)?.license?.$type ||
            (rec.contentRights as any)?.license ||
            "place.stream.metadata.contentRights#all-rights-reserved",
        });
        setLoading(false);
      } catch (err: any) {
        if (cancelled) return;
        // NotFound → fall back to published video
        const vuri = videoUri(did, tid);
        try {
          const existing = await agent.com.atproto.repo.getRecord({
            repo: did,
            collection: "place.stream.video",
            rkey: tid,
          });
          if (cancelled) return;
          const rec = existing.data.value as any;
          setMode("video");
          setVideoUri(vuri);
          setDraftUri(undefined);
          setDraftStatus(undefined);
          setMetaState({
            title: rec.title || "",
            description: rec.description || "",
            activity: rec.activity || undefined,
            tags: rec.tags || [],
            tagInput: "",
            thumbnail: undefined,
            thumbnailUrl: rec.thumb
              ? `https://cdn.stream.place/thumb/${(rec.thumb.ref as any)?.$link || (rec.thumb as any).cid || ""}`
              : undefined,
            warnings: new Set(rec.contentWarnings?.warnings || []),
            license:
              (rec.contentRights as any)?.license?.$type ||
              (rec.contentRights as any)?.license ||
              "place.stream.metadata.contentRights#all-rights-reserved",
          });
          setLoading(false);
        } catch (err2) {
          if (cancelled) return;
          console.error("Failed to load video or draft", err2);
          setError("Could not load this video. It may not exist.");
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [agent, tid]);

  // While the draft is still processing, poll so the editor reflects the
  // processing → ready (or error) transition without a manual reload. Only
  // refreshes the status (and source/durationMs once ready), never the
  // editable metadata the user may be mid-edit on.
  useEffect(() => {
    if (
      mode !== "draft" ||
      draftStatus !== "processing" ||
      !agent ||
      !agent.did
    ) {
      return;
    }
    let cancelled = false;
    const poll = async () => {
      try {
        const res = await agent.place.stream.vod.getDraft({
          uri: draftUri(agent.did!, tid),
        });
        if (cancelled) return;
        const rec = res.data.draft.record as PlaceStreamVodDraftVideo.Main;
        setDraftStatus(rec.status);
      } catch {
        // Transient fetch failures are fine; the next tick retries.
      }
    };
    const interval = setInterval(poll, 3000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [mode, draftStatus, agent, tid]);

  const handleSave = useCallback(async () => {
    if (!agent || !agent.did) return;
    setUpdating(true);
    setError(undefined);
    try {
      if (mode === "draft" && draftUri_) {
        const params: Record<string, any> = {
          uri: draftUri_,
          title: meta.title.trim() || "Untitled",
        };
        if (meta.description.trim())
          params.description = meta.description.trim();
        if (meta.activity) params.activity = meta.activity;
        if (meta.tags.length > 0) params.tags = meta.tags;
        if (meta.warnings.size > 0) {
          params.contentWarnings = {
            $type: "place.stream.metadata.contentWarnings",
            warnings: [...meta.warnings],
          };
        }
        if (
          meta.license &&
          meta.license !==
            "place.stream.metadata.contentRights#all-rights-reserved"
        ) {
          params.contentRights = {
            $type: "place.stream.metadata.contentRights",
            license: meta.license,
          };
        }
        if (meta.thumbnail) {
          try {
            const blobRes = await agent.uploadBlob(meta.thumbnail, {
              encoding: meta.thumbnail.type || "image/jpeg",
            });
            if (blobRes.success) params.thumb = blobRes.data.blob;
          } catch {
            // thumbnail failure is non-fatal
          }
        }
        await agent.place.stream.vod.updateDraft(params as any);
      } else if (mode === "video") {
        // Published video: putRecord. Preserve source/duration/createdAt.
        const existing = await agent.com.atproto.repo.getRecord({
          repo: agent.did,
          collection: "place.stream.video",
          rkey: tid,
        });
        const existingRec = existing.data.value as any;
        const record: Record<string, any> = {
          $type: "place.stream.video",
          title: meta.title.trim(),
          source: existingRec.source,
          durationMs: existingRec.durationMs,
          createdAt: existingRec.createdAt || new Date().toISOString(),
        };
        if (meta.description.trim())
          record.description = meta.description.trim();
        if (meta.activity) record.activity = meta.activity;
        if (meta.tags.length > 0) record.tags = meta.tags;
        if (meta.warnings.size > 0) {
          record.contentWarnings = {
            $type: "place.stream.metadata.contentWarnings",
            warnings: [...meta.warnings],
          };
        }
        if (
          meta.license &&
          meta.license !==
            "place.stream.metadata.contentRights#all-rights-reserved"
        ) {
          record.contentRights = {
            $type: "place.stream.metadata.contentRights",
            license: meta.license,
          };
        }
        if (meta.thumbnail) {
          try {
            const blobRes = await agent.uploadBlob(meta.thumbnail, {
              encoding: meta.thumbnail.type || "image/jpeg",
            });
            if (blobRes.success) record.thumb = blobRes.data.blob;
          } catch {
            // thumbnail is non-fatal
          }
        }
        await agent.com.atproto.repo.putRecord({
          repo: agent.did,
          collection: "place.stream.video",
          rkey: tid,
          record: record as any,
        });
      }
    } catch (err) {
      console.error("Failed to save", err);
      setError(err instanceof Error ? err.message : "Failed to save changes.");
    } finally {
      setUpdating(false);
    }
  }, [agent, mode, draftUri_, tid, meta]);

  const handlePublish = useCallback(async () => {
    if (!agent || !draftUri_) return;
    setPublishing(true);
    setError(undefined);
    try {
      const res = await agent.place.stream.vod.publishDraft({
        uri: draftUri_,
      });
      // Navigate to the player route for the published video.
      const publishedTid = tidFromUri(res.data.videoUri);
      navigation.navigate("Video" as any, {
        user: agent.did ?? "",
        tid: publishedTid,
      });
    } catch (err) {
      console.error("Failed to publish draft", err);
      setError(err instanceof Error ? err.message : "Failed to publish.");
    } finally {
      setPublishing(false);
    }
  }, [agent, draftUri_, navigation]);

  const handleDelete = useCallback(async () => {
    if (!agent || !agent.did) return;
    setDeleting(true);
    setError(undefined);
    try {
      if (mode === "draft" && draftUri_) {
        await agent.place.stream.vod.deleteDraft({ uri: draftUri_ });
      } else if (mode === "video") {
        await agent.com.atproto.repo.deleteRecord({
          repo: agent.did,
          collection: "place.stream.video",
          rkey: tid,
        });
      }
      navigation.navigate("UploadVideos" as any);
    } catch (err) {
      console.error("Failed to delete", err);
      setError(err instanceof Error ? err.message : "Failed to delete.");
    } finally {
      setDeleting(false);
    }
  }, [agent, mode, draftUri_, tid, navigation]);

  if (gate.state !== "ok") {
    return <UploadGate>{null}</UploadGate>;
  }
  if (loading) return <Loading />;
  if (error && !mode) {
    return (
      <ScrollView>
        <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
          <UploadTabNav />
          <View style={{ maxWidth: 960, width: "100%" }}>
            <View style={[zero.p[6], { alignItems: "center", gap: 12 }]}>
              <AlertCircle size={32} color={theme.colors.destructive} />
              <Text color="destructive">{error}</Text>
            </View>
          </View>
        </View>
      </ScrollView>
    );
  }

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <UploadTabNav />

        {/* Editor header */}
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
            {mode === "draft" ? "Editing draft" : "Editing video"}
            {draftStatus
              ? ` · ${draftStatus === "processing" ? "Processing" : draftStatus === "error" ? "Error" : "Ready"}`
              : ""}
          </Text>
          <View style={{ flexDirection: "row", gap: 8 }}>
            <Button
              variant="destructive"
              size="sm"
              onPress={handleDelete}
              disabled={deleting}
            >
              <Text style={{ color: "#fff" }}>
                {deleting ? "Deleting…" : "Delete"}
              </Text>
            </Button>
          </View>
        </View>

        {error && (
          <View style={{ maxWidth: 960, width: "100%", marginBottom: 12 }}>
            <Admonition variant="error" size="sm">
              <Text size="sm">{error}</Text>
            </Admonition>
          </View>
        )}

        <View
          style={{
            maxWidth: 960,
            width: "100%",
            flexDirection: isWide ? "row" : "column",
            alignItems: "flex-start",
            gap: 12,
          }}
        >
          <MetadataEditor meta={meta} setMeta={setMeta} />

          {/* Right column: actions */}
          <View
            style={{ flex: isWide ? 2 : undefined, width: "100%", gap: 12 }}
          >
            <MenuContainer>
              <View style={{ gap: 6 }}>
                <MenuLabel>Actions</MenuLabel>
                <MenuGroup>
                  <View style={[zero.px[3], zero.py[2], { gap: 8 }]}>
                    <Button
                      onPress={handleSave}
                      disabled={!meta.title.trim() || updating}
                    >
                      {updating
                        ? "Saving…"
                        : mode === "draft"
                          ? "Save Draft"
                          : "Update Video"}
                    </Button>
                    {mode === "draft" && draftStatus === "ready" && (
                      <Button
                        onPress={handlePublish}
                        disabled={publishing || updating}
                      >
                        {publishing ? "Publishing…" : "Publish"}
                      </Button>
                    )}
                    {mode === "draft" && draftStatus === "processing" && (
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
                          Processing — publish will be available when ready.
                        </Text>
                      </View>
                    )}
                    {mode === "draft" && draftStatus === "error" && (
                      <Admonition variant="error" size="sm">
                        <Text size="sm">
                          Processing failed. You can discard this draft.
                        </Text>
                      </Admonition>
                    )}
                  </View>
                </MenuGroup>
              </View>
            </MenuContainer>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}

// ── 3. /upload/drafts — drafts list ──────────────────────────────────────────

export function UploadDraftsScreen() {
  const gate = useUploadGate();
  const { theme } = useTheme();
  const navigation = useNavigation();

  const [drafts, setDrafts] = useState<PlaceStreamVodDraftDefs.DraftView[]>([]);
  const [draftsLoading, setDraftsLoading] = useState(false);
  const [draftAction, setDraftAction] = useState<
    Record<string, "publishing" | "deleting" | "error">
  >({});
  // Optimistically-recorded published video URIs: publishDraft returns the new
  // place.stream.video URI, which we surface as a "View VOD" link before the
  // drafts list refresh removes the row server-side.
  const [publishedFromDrafts, setPublishedFromDrafts] = useState<
    Record<string, string>
  >({});

  const agent = gate.state === "ok" ? gate.agent : null;

  const fetchDrafts = useCallback(async () => {
    if (!agent) return;
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

  useEffect(() => {
    if (gate.state === "ok") fetchDrafts();
  }, [gate.state, fetchDrafts]);

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

  if (gate.state !== "ok") {
    return <UploadGate>{null}</UploadGate>;
  }

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <UploadTabNav />

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
                        tid: tidFromUri(publishedUri),
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
                      style={[{ width: "auto" }]}
                      variant="secondary"
                      onPress={() =>
                        navigation.navigate("UploadVideo" as any, {
                          tid: tidFromUri(draft.uri),
                        })
                      }
                    >
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      style={[{ width: "auto" }]}
                      onPress={() => handlePublishDraft(draft.uri)}
                    >
                      Publish
                    </Button>
                    <Button
                      size="sm"
                      style={[{ width: "auto" }]}
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
      </View>
    </ScrollView>
  );
}

// ── 4. /upload/livestreams — livestream list + finalize ──────────────────────

export function UploadLivestreamsScreen() {
  const gate = useUploadGate();
  const { theme } = useTheme();
  const navigation = useNavigation();

  const [livestreams, setLivestreams] = useState<any[]>([]);
  const [livestreamsLoading, setLivestreamsLoading] = useState(false);
  const [userVideos, setUserVideos] = useState<any[]>([]);
  // Per-livestream finalize state. With draft VODs, finalize no longer polls
  // or publishes: it just kicks off the server task and flips the row to
  // "done" so the user knows to find the draft in the Drafts tab.
  const [finalizing, setFinalizing] = useState<
    Record<string, "finalizing" | "done" | "error">
  >({});
  // Livestream URI -> draft ats:// URI, recorded when finalizeLivestream
  // returns so the row can offer a deep-link to the draft editor.
  const [finalizedDraftUris, setFinalizedDraftUris] = useState<
    Record<string, string>
  >({});

  const agent = gate.state === "ok" ? gate.agent : null;

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
    if (gate.state === "ok") {
      fetchVideos();
      fetchLivestreams();
    }
  }, [gate.state, fetchVideos, fetchLivestreams]);

  // Finalize a single livestream into a draft VOD. With draft VODs the
  // client no longer polls getUploadStatus or calls publishVideo: the server
  // kicks off the remux/processing task and creates a draft record (status
  // "processing" → "ready") that the user publishes later from the Drafts tab.
  // We just call finalizeLivestream and flip the row to a "done" state that
  // points the user at the draft editor.
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
        // draft editor.
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

  if (gate.state !== "ok") {
    return <UploadGate>{null}</UploadGate>;
  }

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <UploadTabNav />

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
                No livestreams yet. When you go live, your past streams show up
                here to finalize into VODs.
              </Text>
            </View>
          )}
          {livestreams.map((ls: any) => {
            const rec = ls.value || {};
            const ended = !!rec.endedAt;
            const vodUri = livestreamToVideo.get(ls.uri);
            const status = finalizing[ls.uri];
            const draftU = finalizedDraftUris[ls.uri];
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
                        tid: tidFromUri(vodUri),
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
                    onPress={() => {
                      if (draftU) {
                        navigation.navigate("UploadVideo" as any, {
                          tid: tidFromUri(draftU),
                        });
                      } else {
                        navigation.navigate("UploadDrafts" as any);
                      }
                    }}
                    style={{
                      flexDirection: "row",
                      alignItems: "center",
                      gap: 6,
                    }}
                  >
                    <CheckCircle2 size={16} color={theme.colors.success} />
                    <Text size="sm" style={{ color: theme.colors.success }}>
                      Edit draft →
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
      </View>
    </ScrollView>
  );
}

// ── 5. /upload/videos — My Videos list ───────────────────────────────────────

export function UploadVideosScreen() {
  const gate = useUploadGate();
  const { theme } = useTheme();
  const navigation = useNavigation();

  const [userVideos, setUserVideos] = useState<any[]>([]);

  const agent = gate.state === "ok" ? gate.agent : null;

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
    if (gate.state === "ok") fetchVideos();
  }, [gate.state, fetchVideos]);

  if (gate.state !== "ok") {
    return <UploadGate>{null}</UploadGate>;
  }

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <UploadTabNav />

        <View style={{ maxWidth: 960, width: "100%", gap: 12 }}>
          {userVideos.length === 0 && (
            <View style={[zero.p[6], { alignItems: "center" }]}>
              <Video size={48} color={theme.colors.mutedForeground} />
              <Text
                color="muted"
                size="sm"
                style={{ marginTop: 12, textAlign: "center" }}
              >
                No videos yet. Switch to the Upload tab to add your first video.
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
                onPress={() =>
                  navigation.navigate("UploadVideo" as any, {
                    tid: tidFromUri(video.uri),
                  })
                }
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
      </View>
    </ScrollView>
  );
}
