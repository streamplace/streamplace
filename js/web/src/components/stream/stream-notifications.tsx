import { $Typed } from "@atproto/api";
import type { FacetLink } from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import type { LivestreamStore } from "@streamplace/core";
import { segmentize, type Facet, type FacetFeature } from "@streamplace/core";
import { EyeOff, Pin, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type {
  ChatMessageViewHydrated,
  PinnedRecordViewHydrated,
} from "streamplace";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { useSession } from "../../lib/session";

function rgbColor(
  color?: { red: number; green: number; blue: number } | null,
): string | undefined {
  if (!color) return undefined;
  return `rgb(${color.red}, ${color.green}, ${color.blue})`;
}

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
  comment: PinnedRecordViewHydrated;
}) {
  const { t } = useTranslation("common");
  const { state: sessionState, pdsAgent, did } = useSession();
  const streamerDid = useStore(store, (s) => s.livestream?.author.did);
  const canUnpin = did && streamerDid && did === streamerDid;

  const message = comment.message as ChatMessageViewHydrated | undefined;
  const record = comment.record;
  const authorColor = rgbColor(message?.chatProfile?.color);
  const authorName = message
    ? message.author.displayName || message.author.handle || message.author.did
    : "unknown";
  const messageRecord = message?.record as
    | ChatMessageViewHydrated["record"]
    | undefined;
  const text = messageRecord?.text || "";
  const facets = messageRecord?.facets;

  const segments = facets ? segmentize(text, facets as Facet[]) : [];

  const [dismissed, setDismissed] = useState(false);

  // Handle TTL expiry
  const expiresAt = record.expiresAt ? new Date(record.expiresAt) : null;
  useEffect(() => {
    if (!expiresAt) return;
    const remaining = expiresAt.getTime() - Date.now();
    if (remaining <= 0) {
      setDismissed(true);
      return;
    }
    const timeout = setTimeout(() => setDismissed(true), remaining);
    return () => clearTimeout(timeout);
  }, [expiresAt]);

  // When dismissed, clear from store
  useEffect(() => {
    if (dismissed) {
      store.setState({ pinnedComment: null });
    }
  }, [dismissed, store]);

  const handleDismiss = useCallback(() => {
    setDismissed(true);
  }, []);

  const handleUnpin = useCallback(async () => {
    if (!pdsAgent || !streamerDid) return;
    try {
      const rkey = comment.uri.split("/").pop();
      if (!rkey) return;
      await pdsAgent.com.atproto.repo.deleteRecord({
        repo: streamerDid,
        collection: "place.stream.chat.pinnedRecord",
        rkey,
      });
    } catch (e) {
      console.error("Failed to unpin message:", e);
    }
    store.setState({ pinnedComment: null });
  }, [pdsAgent, streamerDid, comment.uri, store]);

  if (dismissed) return null;

  return (
    <div className="bg-neutral-900 rounded-lg overflow-hidden">
      <div className="flex items-center gap-2 px-3 py-2">
        <div style={{ transform: "rotate(-25deg)" }} className="flex-shrink-0">
          <Pin
            className="w-5 h-5"
            style={{ color: authorColor || "var(--color-accent)" }}
            fill={authorColor || "var(--color-accent)"}
          />
        </div>
        <div className="flex-1 min-w-0 flex flex-col gap-1">
          <div className="flex items-center gap-1 flex-wrap">
            <span
              className="font-semibold text-sm"
              style={{ color: authorColor || "var(--color-fg)" }}
            >
              {authorName}
            </span>
            <span className="text-sm text-neutral-300">
              {segments.length > 0
                ? segments.map((seg, i) => (
                    <PinnedRichText key={i} segment={seg} />
                  ))
                : text}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1 flex-shrink-0">
          {canUnpin && (
            <button
              type="button"
              onClick={handleUnpin}
              className="p-1 rounded hover:bg-white/10 text-neutral-400 hover:text-neutral-200 transition-colors"
              aria-label={t("chat-unpin-message")}
            >
              <X className="w-4 h-4" />
            </button>
          )}
          <button
            type="button"
            onClick={handleDismiss}
            className="p-1 rounded hover:bg-white/10 text-neutral-400 hover:text-neutral-200 transition-colors"
            aria-label={t("chat-dismiss-pinned")}
          >
            <EyeOff className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
}

function PinnedRichText({
  segment,
}: {
  segment: { text: string; features?: unknown[] };
}) {
  const ftr = segment.features?.[0] as FacetFeature | undefined;
  if (!ftr) {
    return <span>{segment.text}</span>;
  }
  if (ftr.$type === "app.bsky.richtext.facet#link") {
    const linkFtr = ftr as $Typed<FacetLink>;
    return (
      <a
        href={linkFtr.uri}
        target="_blank"
        rel="noopener noreferrer"
        className="text-blue-400 hover:underline break-all"
      >
        {segment.text}
      </a>
    );
  }
  if (ftr.$type === "app.bsky.richtext.facet#mention") {
    return <span className="text-blue-400">{segment.text}</span>;
  }
  return <span>{segment.text}</span>;
}

function TeleportNotification({ teleport }: { teleport: any }) {
  const { t } = useTranslation("common");
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
        {t("teleporting-in")}{" "}
        <span className="font-mono font-medium text-[var(--color-fg)]">
          {display}
        </span>
      </p>
    </div>
  );
}
