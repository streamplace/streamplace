import { AlertCircle, CheckCircle, Clock } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Platform, TextInput } from "react-native";
import { useClipStore } from "../../../clip-store";
import { Button, ResponsiveDialog, Text, View, useTheme } from "../../ui";
import { ClipEditorWindow } from "./clip-editor-window";
import { ClipPreview } from "./clip-preview";
import { TrimTimeline } from "./trim-timeline";

function formatCountdown(ms: number): string {
  const totalSeconds = Math.ceil(Math.max(0, ms) / 1000);
  const m = Math.floor(totalSeconds / 60);
  const s = totalSeconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

// The clip editor, mounted at the player screen as a sibling of the player —
// outside MobileUi's fade overlay and gesture detector. Presentation is
// platform-split: a draggable overlay window on web (see ClipEditorWindow) and
// a full-screen bottom-sheet takeover on native (ResponsiveDialog). Both share
// the same ClipEditorBody, which renders every draft state: editing,
// publishing, published, expired, and publish errors.
export function ClipEditorModal() {
  const status = useClipStore((s) => s.status);
  const clipId = useClipStore((s) => s.clipId);
  const cancel = useClipStore((s) => s.cancel);

  const isPublished = status === "published";
  const isExpired = status === "expired";
  const isPublishing = status === "publishing";
  // status "error" with a clipId means a publish failure — keep the editor
  // open with an inline banner. Create failures (no clipId) are toasted and
  // reset to idle by the trigger, so they never reach the editor.
  const isPublishError = status === "error" && clipId !== null;
  const isOpen =
    status === "editing" ||
    isPublishing ||
    isPublished ||
    isExpired ||
    isPublishError;

  const title = isPublished
    ? "Clip published"
    : isExpired
      ? "Clip expired"
      : "Create clip";

  if (Platform.OS === "web") {
    return isOpen ? (
      <ClipEditorWindow title={title} onClose={cancel}>
        <ClipEditorBody />
      </ClipEditorWindow>
    ) : null;
  }

  return (
    <ResponsiveDialog open={isOpen} onClose={cancel} title={title}>
      <ClipEditorBody />
    </ResponsiveDialog>
  );
}

function ClipEditorBody() {
  const th = useTheme();
  const status = useClipStore((s) => s.status);
  const clipId = useClipStore((s) => s.clipId);
  const previewUrl = useClipStore((s) => s.previewUrl);
  const durationMs = useClipStore((s) => s.durationMs);
  const trimStart = useClipStore((s) => s.trimStart);
  const trimEnd = useClipStore((s) => s.trimEnd);
  const title = useClipStore((s) => s.title);
  const timeRemaining = useClipStore((s) => s.timeRemaining);
  const error = useClipStore((s) => s.error);
  const videoUri = useClipStore((s) => s.videoUri);
  const setTitle = useClipStore((s) => s.setTitle);
  const publish = useClipStore((s) => s.publish);
  const cancel = useClipStore((s) => s.cancel);
  const discard = useClipStore((s) => s.discard);

  const [playheadMs, setPlayheadMs] = useState(0);
  const [seekTo, setSeekTo] = useState<number | null>(null);
  const [copied, setCopied] = useState(false);

  const isPublishing = status === "publishing";
  const isPublished = status === "published";
  const isExpired = status === "expired";
  const isPublishError = status === "error" && clipId !== null;

  // After a trim commit, point the preview at the new range start instead of
  // the last manual seek.
  useEffect(() => {
    setSeekTo(null);
  }, [trimStart, trimEnd]);

  const copyLink = async () => {
    if (Platform.OS !== "web" || !videoUri) return;
    try {
      await navigator.clipboard.writeText(videoUri);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard unavailable (permissions/HTTPS) — leave the copy button as-is.
    }
  };

  const lowTime = timeRemaining < 60000;
  const countdownColor = lowTime ? "#f59e0b" : th.theme.colors.primary;

  if (isPublished) {
    return (
      <View style={{ gap: 12, paddingVertical: 8 }}>
        <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
          <CheckCircle
            size={20}
            color={th.theme.colors.success ?? th.theme.colors.primary}
          />
          <Text size="sm" weight="semibold">
            Your clip is live!
          </Text>
        </View>
        {Platform.OS === "web" ? (
          <Button variant="secondary" onPress={copyLink}>
            {copied ? "Copied!" : "Copy link"}
          </Button>
        ) : (
          <View style={{ gap: 4 }}>
            <Text size="xs" color="muted">
              Clip URL
            </Text>
            <Text size="xs" numberOfLines={2} ellipsizeMode="middle">
              {videoUri}
            </Text>
          </View>
        )}
        <Button variant="primary" onPress={discard}>
          Done
        </Button>
      </View>
    );
  }

  if (isExpired) {
    return (
      <View style={{ gap: 12, paddingVertical: 8 }}>
        <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}>
          <Clock size={20} color={th.theme.colors.foreground} />
          <Text size="sm" weight="semibold">
            Your clip has expired
          </Text>
        </View>
        <Text size="sm" color="muted">
          The draft is gone — start a new clip to try again.
        </Text>
        <Button variant="primary" onPress={discard}>
          Done
        </Button>
      </View>
    );
  }

  return (
    <View style={{ gap: 12 }}>
      {/* Preview */}
      <View
        style={{
          aspectRatio: 16 / 9,
          borderRadius: 8,
          overflow: "hidden",
          backgroundColor: "#000",
        }}
      >
        {previewUrl ? (
          <ClipPreview
            uri={previewUrl}
            trimStart={trimStart}
            trimEnd={trimEnd}
            seekTo={seekTo ?? undefined}
            onTimeUpdate={setPlayheadMs}
          />
        ) : null}
      </View>

      {/* Trim timeline */}
      {durationMs > 0 && (
        <TrimTimeline onSeek={setSeekTo} playheadMs={playheadMs} />
      )}

      {/* Countdown */}
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: 6,
          padding: 8,
          borderRadius: 8,
          backgroundColor: th.theme.colors.secondary,
        }}
      >
        <Clock size={14} color={countdownColor} />
        <Text size="xs" style={{ color: countdownColor }}>
          expires in {formatCountdown(timeRemaining)}
        </Text>
      </View>

      {/* Title */}
      <View style={{ gap: 4 }}>
        <Text size="xs" weight="semibold">
          Title
        </Text>
        <TextInput
          value={title}
          onChangeText={setTitle}
          placeholder="Add a title..."
          placeholderTextColor={th.theme.colors.mutedForeground}
          maxLength={140}
          editable={!isPublishing}
          style={{
            padding: 8,
            borderRadius: 8,
            borderWidth: 1,
            borderColor: th.theme.colors.border,
            color: th.theme.colors.foreground,
          }}
        />
      </View>

      {/* Publish error banner */}
      {isPublishError && error && (
        <View
          style={{
            flexDirection: "row",
            alignItems: "center",
            gap: 6,
            padding: 8,
            borderRadius: 8,
            backgroundColor: "#fef2f2",
          }}
        >
          <AlertCircle size={14} color="#dc2626" />
          <Text size="xs" style={{ color: "#dc2626", flex: 1 }}>
            {error}
          </Text>
        </View>
      )}

      {/* Actions */}
      <View style={{ flexDirection: "row", gap: 8 }}>
        <View style={{ flex: 1 }}>
          <Button variant="secondary" onPress={cancel} disabled={isPublishing}>
            Cancel
          </Button>
        </View>
        <View style={{ flex: 1 }}>
          <Button
            variant="primary"
            onPress={publish}
            loading={isPublishing}
            loadingText="Publishing..."
            disabled={!title.trim()}
          >
            Publish
          </Button>
        </View>
      </View>
    </View>
  );
}
