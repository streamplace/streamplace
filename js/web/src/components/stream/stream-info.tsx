import type { LivestreamStore } from "@streamplace/core";
import type { PlaceStreamDefs } from "streamplace";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import type { Liveness } from "../../hooks/use-liveness-state";
import { formatViewers } from "../../lib/format";
import { useSession } from "../../lib/session";

const ACTIVITY_LABELS: Record<string, string> = {
  events: "Events",
  just_chatting: "Just Chatting",
  music: "Music",
  art: "Art",
  software_dev: "Software Dev",
  cooking: "Cooking",
  miniatures: "Miniatures",
  makers_crafting: "Makers & Crafting",
  fitness: "Fitness",
  sports: "Sports",
};

export function activityLabel(
  activity:
    | PlaceStreamDefs.ActivityGame
    | PlaceStreamDefs.ActivityLabel
    | { $type: string }
    | undefined,
): string | null {
  if (!activity) return null;
  if ("name" in activity && activity.name) return activity.name;
  if ("label" in activity)
    return ACTIVITY_LABELS[activity.label] ?? activity.label;
  return null;
}

export function StreamInfo({
  store,
  user,
  liveness,
  chatOpen,
  onToggleChat,
}: {
  store: LivestreamStore;
  user: string;
  liveness: Liveness;
  chatOpen: boolean;
  onToggleChat: () => void;
}) {
  const state = useStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
      viewers: s.viewers,
    })),
  );

  const { state: sessionState } = useSession();
  const record = state.livestream?.record;
  const author = state.livestream?.author;
  const title = record?.title || user;
  const activity = activityLabel(record?.activity);
  const tags = record?.tags;
  const viewers = formatViewers(state.viewers);
  const isLive = liveness === "live";

  return (
    <div className="mt-3 space-y-3 mx-3">
      <div className="flex items-start gap-3">
        <img
          src={author?.avatar ?? undefined}
          alt=""
          className="w-10 h-10 rounded-full bg-[var(--color-bg-elevated)] flex-shrink-0"
          onError={(e) => {
            (e.currentTarget as HTMLImageElement).style.display = "none";
          }}
        />

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="truncate">
              {author?.displayName || author?.handle || user}
            </span>
            {isLive && viewers && (
              <span className="text-xs text-[var(--color-fg-muted)] flex-shrink-0">
                {viewers} watching
              </span>
            )}
          </div>

          <h2 className="font-semibold text-[var(--color-fg)] mt-0.5 line-clamp-2">
            {title}
          </h2>

          <div className="flex items-center gap-1.5 mt-1.5 flex-wrap">
            {activity && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-[var(--color-bg-elevated)] border border-[var(--color-border)] text-[var(--color-fg-muted)]">
                {activity}
              </span>
            )}
            {tags?.map((tag) => (
              <span
                key={tag}
                className="text-xs px-2 py-0.5 rounded-full bg-[var(--color-bg-elevated)] border border-[var(--color-border)] text-[var(--color-fg-subtle)]"
              >
                {tag.startsWith("lang:") ? tag.slice(5).toUpperCase() : tag}
              </span>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-2 flex-shrink-0">
          {sessionState.status === "authenticated" &&
            sessionState.session.did !== author?.did && (
              <button
                type="button"
                className="h-8 px-3 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-[var(--color-accent-fg)] text-xs font-medium"
              >
                Follow
              </button>
            )}
          <button
            type="button"
            className="h-8 px-3 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-xs text-[var(--color-fg-muted)]"
            onClick={() => {
              navigator.clipboard
                .writeText(window.location.href)
                .catch(() => {});
            }}
          >
            Share
          </button>
          <button
            type="button"
            className="h-8 px-3 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-xs text-[var(--color-fg-muted)]"
            onClick={onToggleChat}
          >
            {chatOpen ? "Hide chat" : "Chat"}
          </button>
        </div>
      </div>
    </div>
  );
}
