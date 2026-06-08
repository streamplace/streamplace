import type { LivestreamStore } from "@streamplace/core";
import { Pin } from "lucide-react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { useSession } from "../../lib/session";

export function StreamNotifications({ store }: { store: LivestreamStore }) {
  const state = useStore(
    store,
    useShallow((s) => ({
      pinnedComment: s.pinnedComment,
      activeTeleport: s.activeTeleport,
    })),
  );

  return (
    <>
      {state.pinnedComment && (
        <PinnedNotification store={store} comment={state.pinnedComment} />
      )}
      {state.activeTeleport && (
        <TeleportNotification teleport={state.activeTeleport} />
      )}
    </>
  );
}

function PinnedNotification({
  store,
  comment,
}: {
  store: LivestreamStore;
  comment: any;
}) {
  const { state } = useSession();
  const isOwn =
    state.status === "authenticated" &&
    state.session.did === comment.author?.did;

  const text = comment.record?.text;

  return (
    <div className="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-overlay)]">
      <div className="flex items-center gap-1.5 mb-0.5">
        <Pin className="w-3 h-3 text-[var(--color-accent)]" />
        <span className="text-xs font-medium text-[var(--color-fg-muted)] uppercase tracking-wide">
          Pinned message
          {isOwn && " · You"}
        </span>
      </div>
      <p className="text-sm text-[var(--color-fg)] line-clamp-2">
        {text || ""}
      </p>
    </div>
  );
}

function TeleportNotification({ teleport }: { teleport: any }) {
  const targetDid = teleport.target;
  const startsAt = teleport.startsAt
    ? new Date(teleport.startsAt).getTime()
    : null;

  if (!startsAt) return null;

  const now = Date.now();
  const diff = Math.max(0, Math.ceil((startsAt - now) / 1000));

  if (diff <= 0) return null;

  const mins = Math.floor(diff / 60);
  const secs = diff % 60;
  const display =
    mins > 0
      ? `${mins}:${String(secs).padStart(2, "0")}`
      : `0:${String(secs).padStart(2, "0")}`;

  return (
    <div className="px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-info)]/10">
      <p className="text-xs text-[var(--color-fg-muted)]">
        Teleporting in{" "}
        <span className="font-mono font-medium text-[var(--color-fg)]">
          {display}
        </span>
      </p>
    </div>
  );
}
