import { getStreamplaceUrl } from "@/lib/streamplace-url";
import type { LivestreamStore } from "@streamplace/core";
import { Eye, EyeOff, Radio, Wifi, WifiOff } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { Player } from "../player/player";

/**
 * Self-preview video monitor for the dashboard. Shows the streamer's
 * own stream with connection status, stream title, and a hide/show
 * toggle. Port of StreamMonitor from the RN app.
 */
export function StreamMonitorWidget({
  store,
  user,
}: {
  store: LivestreamStore;
  user: string;
}) {
  const { t } = useTranslation("common");
  const state = useStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
      segment: s.segment,
      websocketConnected: s.websocketConnected,
    })),
  );

  const [visible, setVisible] = useState(true);

  const isLive = !!state.livestream;
  const title = state.livestream?.record?.title || null;

  const playlistUrl = useMemo(
    () =>
      `${getStreamplaceUrl()}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(user)}`,
    [user],
  );

  const thumbnailUrl = useMemo(
    () =>
      `${getStreamplaceUrl()}/api/playback/${encodeURIComponent(user)}/stream.jpg`,
    [user],
  );

  const connectionIcon = isLive ? (
    state.websocketConnected ? (
      <Wifi className="size-3.5 text-green-400" />
    ) : (
      <WifiOff className="size-3.5 text-red-400" />
    )
  ) : (
    <Wifi className="size-3.5 text-[var(--color-fg-muted)]" />
  );

  const statusLabel = isLive
    ? t("live-badge", { defaultValue: "LIVE" })
    : t("offline", { defaultValue: "OFFLINE" });

  return (
    <div className="flex h-full flex-col overflow-hidden rounded-b-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)]">
      {/* Video area */}
      <div className="relative h-full w-full flex-1 bg-black">
        {visible && isLive ? (
          <Player src={playlistUrl} poster={thumbnailUrl} active mode="live" />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center">
            {isLive ? (
              <img
                src={thumbnailUrl}
                alt=""
                className="absolute inset-0 h-full w-full object-contain"
                onError={(e) => {
                  (e.currentTarget as HTMLImageElement).style.visibility =
                    "hidden";
                }}
              />
            ) : (
              <div className="text-center">
                <Radio className="mx-auto mb-2 size-8 text-[var(--color-fg-muted)]" />
                <p className="text-sm text-[var(--color-fg-muted)]">
                  {t("stream-offline", {
                    defaultValue: "Stream is offline",
                  })}
                </p>
              </div>
            )}
          </div>
        )}

        {/* LIVE badge */}
        {isLive && (
          <div className="pointer-events-none absolute top-2 left-2 z-10">
            <div className="flex items-center gap-1 rounded bg-red-600 px-1.5 py-0.5 text-[10px] font-bold tracking-wide text-white uppercase">
              <div className="h-1.5 w-1.5 rounded-full bg-white" />
              {t("live-badge", { defaultValue: "LIVE" })}
            </div>
          </div>
        )}
      </div>

      {/* Info bar */}
      <div className="flex items-center justify-between border-t border-[var(--color-border)] px-3 py-2">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          {connectionIcon}
          <span className="truncate text-sm">
            {title || t("untitled-stream", { defaultValue: "Untitled Stream" })}
          </span>
        </div>

        <div className="flex flex-shrink-0 items-center gap-2">
          {isLive && (
            <button
              type="button"
              onClick={() => setVisible((v) => !v)}
              className="rounded p-1 text-[var(--color-fg-muted)] transition-colors hover:bg-[var(--color-bg)] hover:text-[var(--color-fg)]"
              title={
                visible
                  ? t("hide-stream", { defaultValue: "Hide stream" })
                  : t("show-stream", { defaultValue: "Show stream" })
              }
            >
              {visible ? (
                <EyeOff className="size-3.5" />
              ) : (
                <Eye className="size-3.5" />
              )}
            </button>
          )}
          <span
            className={`text-xs font-medium tracking-wider uppercase ${
              isLive ? "text-[var(--color-fg)]" : "text-[var(--color-fg-muted)]"
            }`}
          >
            {statusLabel}
          </span>
        </div>
      </div>
    </div>
  );
}
