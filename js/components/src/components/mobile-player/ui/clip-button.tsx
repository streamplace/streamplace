import { Clapperboard, Loader2, Lock } from "lucide-react-native";
import { useEffect, useRef } from "react";
import { Pressable } from "react-native";
import { useClipStore } from "../../../clip-store";
import { useAuthor } from "../../../hooks/useAuthor";
import { useLivestreamStoreOptional } from "../../../livestream-store";
import { useStreamplaceStore } from "../../../streamplace-store";
import { Text, toast, useTheme } from "../../ui";

// The trigger for the clip editor: a clapperboard while idle, a spinner while
// a clip is being requested, and a login CTA for logged-out viewers. It never
// renders the editor — the ClipEditorModal (mounted at the player screen) is
// the only thing that does.
export function ClipButton({
  onLoginRequired,
}: {
  /** Called when a logged-out viewer presses the login CTA. */
  onLoginRequired?: () => void;
}) {
  const th = useTheme();
  const profile = useAuthor();
  const oauthSession = useStreamplaceStore((x) => x.oauthSession);
  const livestreamUri = useLivestreamStoreOptional(
    (x) => x.livestream?.uri ?? null,
  );

  const status = useClipStore((s) => s.status);
  const clipId = useClipStore((s) => s.clipId);
  const error = useClipStore((s) => s.error);
  const requestClip = useClipStore((s) => s.requestClip);
  const discard = useClipStore((s) => s.discard);

  // When a *create* request fails (no clipId yet) the button surfaces the
  // mapped error as a toast and returns to idle. Publish failures (clipId set)
  // are handled inline by the editor, which stays open.
  const prevStatus = useRef(status);
  useEffect(() => {
    if (prevStatus.current !== "error" && status === "error" && !clipId) {
      if (error) {
        toast.show("Couldn't create clip", error, {
          variant: "error",
          duration: 4,
        });
      }
      discard();
    }
    prevStatus.current = status;
  }, [status, error, clipId, discard]);

  // Logged out: render a login CTA instead of the clapperboard. The session
  // can also be undefined while it's still loading — that's not "logged out",
  // so keep the clapperboard (disabled) in that case.
  if (oauthSession === null) {
    return (
      <Pressable
        onPress={onLoginRequired}
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: 6,
          paddingVertical: 8,
          paddingHorizontal: 10,
          backgroundColor: "rgba(90,90,90, 0.3)",
          borderRadius: 12,
        }}
      >
        <Lock size={14} color={th.theme.colors.foreground} />
        <Text size="xs">Log in to clip</Text>
      </Pressable>
    );
  }

  const isLoading = status === "requesting";
  const canClip = profile?.did != null && oauthSession !== undefined;

  const handlePress = () => {
    if (!profile?.did || oauthSession === undefined) return;
    requestClip({
      streamerDID: profile.did,
      oauthSession,
      livestreamUri,
    });
  };

  return (
    <Pressable
      onPress={handlePress}
      disabled={isLoading || !canClip}
      style={{ padding: 8, opacity: canClip ? 1 : 0.5 }}
    >
      {isLoading ? (
        <Loader2 size={20} color={th.theme.colors.primary} />
      ) : (
        <Clapperboard size={20} color={th.theme.colors.foreground} />
      )}
    </Pressable>
  );
}
