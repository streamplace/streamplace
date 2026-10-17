// Standalone chat popout window. Minimal chrome; just the chat panel
// and input, no player, no sidebar, no header.
//
// Query parameters (useful for OBS browser sources):
//   reverse=true     Show newest messages at top
//   reverse=false    Show newest messages at bottom (default)
//   hideAfter=N      Hide the chat after N seconds
//   hideChatBox=true Hide the chat input (read-only)
//   hidePinnedComments=true  Hide pinned comment notifications
import { ChatInput } from "@/components/stream/chat-input";
import { ChatPanel } from "@/components/stream/chat-panel";
import { StreamNotifications } from "@/components/stream/stream-notifications";
import { useLivestreamStore } from "@/hooks/use-livestream-store";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";

export const Route = createFileRoute("/chat-popout/$user")({
  validateSearch: (
    search: Record<string, unknown>,
  ): {
    reverse?: boolean;
    hideAfter?: number;
    hideChatBox?: boolean;
    hidePinnedComments?: boolean;
  } => {
    const toBool = (v: unknown): boolean | undefined => {
      if (v === undefined || v === null) return undefined;
      if (typeof v === "boolean") return v;
      return String(v).toLowerCase() === "true";
    };
    const toNum = (v: unknown): number | undefined => {
      if (typeof v === "number" && !isNaN(v)) return v;
      if (typeof v === "string" && !isNaN(Number(v))) return Number(v);
      return undefined;
    };
    const result: {
      reverse?: boolean;
      hideAfter?: number;
      hideChatBox?: boolean;
      hidePinnedComments?: boolean;
    } = {};
    const reverse = toBool(search.reverse);
    if (reverse !== undefined) result.reverse = reverse;
    const hideAfter = toNum(search.hideAfter);
    if (hideAfter !== undefined) result.hideAfter = hideAfter;
    const hideChatBox = toBool(search.hideChatBox);
    if (hideChatBox !== undefined) result.hideChatBox = hideChatBox;
    const hidePinnedComments = toBool(search.hidePinnedComments);
    if (hidePinnedComments !== undefined)
      result.hidePinnedComments = hidePinnedComments;
    return result;
  },
  component: ChatPopoutPage,
});

function ChatPopoutPage() {
  const { user } = Route.useParams();
  const { reverse, hideAfter, hideChatBox, hidePinnedComments } =
    Route.useSearch();
  const reverseVal = reverse ?? false;
  const hideChatBoxVal = hideChatBox ?? false;
  const hidePinnedCommentsVal = hidePinnedComments ?? false;
  const { store, ready } = useLivestreamStore(user);
  const [hidden, setHidden] = useState(false);

  // hideAfter timer
  useEffect(() => {
    if (!hideAfter || hideAfter <= 0) return;
    const timer = setTimeout(() => setHidden(true), hideAfter * 1000);
    return () => clearTimeout(timer);
  }, [hideAfter]);

  if (!ready || !store) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-(--color-border) border-t-(--color-accent)" />
      </div>
    );
  }

  return (
    <div
      className="flex h-screen flex-col bg-(--color-background)"
      style={
        hidden ? { opacity: 0, pointerEvents: "none" as const } : undefined
      }
    >
      {!hidePinnedCommentsVal && <StreamNotifications store={store} />}
      <div className="flex min-h-0 flex-1 flex-col">
        <ChatPanel store={store} reversed={reverseVal} />
      </div>
      {!hideChatBoxVal && (
        <div className="border-t p-2">
          <ChatInput store={store} />
        </div>
      )}
    </div>
  );
}
