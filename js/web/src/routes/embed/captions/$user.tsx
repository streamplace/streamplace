import { useLivestreamStore } from "@/hooks/use-livestream-store";
import type { LivestreamStore } from "@streamplace/core";
import {
  CAPTION_WEB_FONTS,
  captionColor,
  captionFontSize,
  DEFAULT_CAPTION_PREFS,
  displayLiveCaptions,
  parseCaptionPrefs,
} from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";

export const Route = createFileRoute("/embed/captions/$user")({
  component: CaptionOverlay,
  validateSearch: (search: Record<string, unknown>) => ({
    fontSize:
      typeof search.fontSize === "string" || typeof search.fontSize === "number"
        ? Number(search.fontSize)
        : undefined,
    color: typeof search.color === "string" ? search.color : undefined,
    background:
      typeof search.background === "string" ? search.background : undefined,
    position:
      search.position === "top" || search.position === "center"
        ? search.position
        : "bottom",
    maxLines: Math.min(10, Math.max(1, Number(search.maxLines) || 2)),
    track: typeof search.track === "string" ? search.track : undefined,
    font: typeof search.font === "string" ? search.font : undefined,
  }),
});

function CaptionOverlay() {
  const { user } = Route.useParams();
  const { store } = useLivestreamStore(user);
  useEffect(() => {
    const body = document.body.style.background;
    const root = document.documentElement.style.background;
    document.body.style.background = "transparent";
    document.documentElement.style.background = "transparent";
    return () => {
      document.body.style.background = body;
      document.documentElement.style.background = root;
    };
  }, []);
  return store ? (
    <OverlayBody store={store} />
  ) : (
    <div className="fixed inset-0 bg-transparent" />
  );
}

function OverlayBody({ store }: { store: LivestreamStore }) {
  const search = Route.useSearch();
  const { t } = useTranslation("common");
  const tracks = useStore(store, (state) => state.captionTracks);
  const captions = useStore(store, (state) => state.liveCaptions);
  const [now, setNow] = useState(Date.now());
  const [height, setHeight] = useState(window.innerHeight);
  useEffect(() => {
    const tick = window.setInterval(() => setNow(Date.now()), 250);
    const resize = () => setHeight(window.innerHeight);
    window.addEventListener("resize", resize);
    return () => {
      window.clearInterval(tick);
      window.removeEventListener("resize", resize);
    };
  }, []);
  const track =
    search.track ??
    tracks.find((item) => item.origin === "canonical")?.id ??
    tracks[0]?.id;
  const lines = track ? displayLiveCaptions(captions, track, now) : [];
  const prefs = parseCaptionPrefs(
    JSON.stringify({ ...DEFAULT_CAPTION_PREFS, font: search.font }),
  );
  const fontSize =
    Number.isFinite(search.fontSize) && (search.fontSize ?? 0) > 0
      ? Math.min(200, search.fontSize!)
      : `max(var(--text-lg), ${captionFontSize(prefs, height, 0)}px)`;
  const color =
    search.color && CSS.supports("color", search.color)
      ? search.color
      : captionColor(prefs.textColor, prefs.textOpacity);
  const background =
    search.background && CSS.supports("color", search.background)
      ? search.background
      : captionColor(prefs.backgroundColor, prefs.backgroundOpacity);
  return (
    <div
      className="pointer-events-none fixed inset-0 flex flex-col items-center bg-transparent p-6"
      style={{
        justifyContent:
          search.position === "top"
            ? "flex-start"
            : search.position === "center"
              ? "center"
              : "flex-end",
      }}
    >
      <div
        aria-live="polite"
        aria-label={t("player-captions")}
        className="max-w-full rounded-md px-4 py-2 text-center"
        style={{
          color,
          background: lines.length ? background : "transparent",
          fontSize,
          fontFamily: CAPTION_WEB_FONTS[prefs.font],
          display: "-webkit-box",
          WebkitBoxOrient: "vertical",
          WebkitLineClamp: search.maxLines,
          overflow: "hidden",
        }}
      >
        {lines.map((line) => (
          <span
            key={line.id}
            className="block"
            style={{ fontSize: "inherit", fontFamily: "inherit" }}
          >
            {line.text}
          </span>
        ))}
      </div>
    </div>
  );
}
