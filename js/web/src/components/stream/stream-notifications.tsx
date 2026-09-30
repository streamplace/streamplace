import { $Typed } from "@atproto/api";
import type { Link as FacetLink } from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import type { LivestreamStore } from "@streamplace/core";
import {
  deleteTeleport,
  segmentize,
  type Facet,
  type FacetFeature,
} from "@streamplace/core";
import { EyeOff, Pin, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type {
  ChatMessageViewHydrated,
  PinnedRecordViewHydrated,
} from "streamplace";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import useAvatars from "../../hooks/use-avatars";
import { useCanModerate } from "../../hooks/use-can-moderate";
import { useModerationActions } from "../../hooks/use-moderation-actions";
import { useSession } from "../../lib/session";
import { streamNotification } from "../../lib/stream-notification";

function rgbColor(
  color?: { red: number; green: number; blue: number } | null,
): string | undefined {
  if (!color) return undefined;
  return `rgb(${color.red}, ${color.green}, ${color.blue})`;
}

export function StreamNotifications({
  store,
  showTeleport = true,
}: {
  store: LivestreamStore;
  showTeleport?: boolean;
}) {
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
      {showTeleport && state.activeTeleport && (
        <TeleportNotification store={store} teleport={state.activeTeleport} />
      )}
    </>
  );
}

export function StreamTeleportNotification({
  store,
}: {
  store: LivestreamStore;
}) {
  const teleport = useStore(store, (s) => s.activeTeleport);

  useEffect(() => {
    if (!teleport) {
      streamNotification.hide("teleport");
      return;
    }

    streamNotification.show({
      id: "teleport",
      duration: 0,
      render: () => <TeleportNotification store={store} teleport={teleport} />,
    });
    return () => streamNotification.hide("teleport");
  }, [store, teleport]);

  return null;
}

function PinnedNotification({
  store,
  comment,
}: {
  store: LivestreamStore;
  comment: PinnedRecordViewHydrated;
}) {
  const { t } = useTranslation("common");
  const { did } = useSession();
  const streamerDid = useStore(store, (s) => s.livestream?.author.did);
  const moderation = useCanModerate(store);
  const { unpinMessage } = useModerationActions();
  const [unpinning, setUnpinning] = useState(false);
  const canUnpin = !!did && moderation.canPin;

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
    if (!streamerDid || unpinning) return;
    setUnpinning(true);
    try {
      await unpinMessage(comment.uri, streamerDid);
      store.setState({ pinnedComment: null });
    } catch (e) {
      console.error("Failed to unpin message:", e);
    } finally {
      setUnpinning(false);
    }
  }, [streamerDid, unpinMessage, unpinning, comment.uri, store]);

  if (dismissed) return null;

  return (
    <div className="overflow-hidden rounded-lg bg-neutral-900">
      <div className="flex items-center gap-2 px-3 py-2">
        <div style={{ transform: "rotate(-25deg)" }} className="shrink-0">
          <Pin
            className="h-5 w-5"
            style={{ color: authorColor || "var(--color-accent)" }}
            fill={authorColor || "var(--color-accent)"}
          />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex flex-wrap items-center gap-1">
            <span
              className="text-sm font-semibold"
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
        <div className="flex shrink-0 items-center gap-1">
          {canUnpin && (
            <button
              type="button"
              onClick={handleUnpin}
              disabled={unpinning}
              className="rounded p-1 text-neutral-400 transition-colors hover:bg-white/10 hover:text-neutral-200"
              aria-label={t("chat-unpin-message")}
            >
              <X className="h-4 w-4" />
            </button>
          )}
          <button
            type="button"
            onClick={handleDismiss}
            className="rounded p-1 text-neutral-400 transition-colors hover:bg-white/10 hover:text-neutral-200"
            aria-label={t("chat-dismiss-pinned")}
          >
            <EyeOff className="h-4 w-4" />
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
        className="break-all text-blue-400 hover:underline"
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

function TeleportNotification({
  store,
  teleport,
}: {
  store: LivestreamStore;
  teleport: NonNullable<ReturnType<typeof store.getState>["activeTeleport"]>;
}) {
  const { t } = useTranslation("common");
  const { pdsAgent, did } = useSession();
  const targetProfiles = useAvatars([teleport.streamer]);
  const { activeTeleportUri, livestream } = useStore(
    store,
    useShallow((s) => ({
      activeTeleportUri: s.activeTeleportUri,
      livestream: s.livestream,
    })),
  );
  const [now, setNow] = useState(Date.now());
  const startsAt = new Date(teleport.startsAt).getTime();
  const targetHandle =
    targetProfiles[teleport.streamer]?.handle || teleport.streamer;
  const initialTimeLeft = useMemo(
    () => Math.max(0, startsAt - Date.now()),
    [startsAt],
  );
  const canCancel = !!did && did === livestream?.author.did;

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);

  const cancel = async () => {
    if (!pdsAgent || !did || !activeTeleportUri) return;
    try {
      await deleteTeleport(pdsAgent, did, activeTeleportUri);
      store.setState({ activeTeleportUri: null, activeTeleport: null });
    } catch (error) {
      console.error("Failed to cancel teleport:", error);
    }
  };

  if (!Number.isFinite(startsAt)) return null;

  const diff = Math.max(0, Math.ceil((startsAt - now) / 1000));

  useEffect(() => {
    if (diff <= 0) streamNotification.hide("teleport");
  }, [diff]);

  if (diff <= 0) return null;

  const mins = Math.floor(diff / 60);
  const secs = diff % 60;
  const display =
    mins > 0
      ? `${mins}:${String(secs).padStart(2, "0")}`
      : `0:${String(secs).padStart(2, "0")}`;

  return (
    <div className="relative isolate overflow-hidden rounded-lg bg-(--color-bg-elevated) text-(--color-fg)">
      <div className="relative z-10 flex items-center justify-between gap-3 px-3 py-2">
        <p className="min-w-0 truncate text-sm font-medium">
          {t("teleporting-to", { handle: targetHandle })}
        </p>
        <div className="flex shrink-0 items-center gap-3">
          <span className="font-mono text-sm text-(--color-fg-muted)">
            {display}
          </span>
          {canCancel && activeTeleportUri && (
            <button
              type="button"
              onClick={cancel}
              className="rounded px-2 py-1 text-xs text-(--color-fg-muted) hover:bg-(--color-bg-overlay) hover:text-(--color-fg) focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--color-accent)"
            >
              {t("cancel")}
            </button>
          )}
        </div>
      </div>
      <div
        className="h-1 overflow-hidden bg-(--color-bg)"
        role="progressbar"
        aria-label={t("teleporting-in")}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={
          initialTimeLeft > 0
            ? Math.round((Math.max(0, startsAt - now) / initialTimeLeft) * 100)
            : 0
        }
      >
        <div
          className="h-full bg-(--color-accent) transition-[width] duration-1000 ease-linear motion-reduce:transition-none"
          style={{
            width: `${
              initialTimeLeft > 0
                ? Math.max(
                    0,
                    Math.min(100, ((startsAt - now) / initialTimeLeft) * 100),
                  )
                : 0
            }%`,
          }}
        />
      </div>
      <div
        className="teleport-warning-stripes pointer-events-none absolute inset-0 z-0 motion-reduce:hidden"
        aria-hidden="true"
      />
    </div>
  );
}
