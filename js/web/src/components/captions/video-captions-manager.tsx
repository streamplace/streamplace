import { usePDSAgent, useStreamplaceUrl } from "@/lib/store/hooks";
import { isValidAtUri } from "@atproto/syntax";
import {
  captionLanguageName,
  captionsUrl,
  fetchCaptionTracks,
  type CaptionTrackView,
} from "@streamplace/core";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import { Input } from "../ui/input";

export function VideoCaptionsManager({ video }: { video: string }) {
  const agent = usePDSAgent();
  const url = useStreamplaceUrl();
  const { t, i18n } = useTranslation();
  const [tracks, setTracks] = useState<CaptionTrackView[]>([]);
  const [language, setLanguage] = useState("en");
  const [reload, setReload] = useState(0);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    fetchCaptionTracks(url, { video }, controller.signal)
      .then(setTracks)
      .catch((error) => {
        if (!controller.signal.aborted) setMessage(error.message);
      });
    return () => controller.abort();
  }, [url, video, reload]);
  const upload = async (file: File) => {
    if (!agent || !isValidAtUri(video)) return;
    if (!/^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$/.test(language.trim())) {
      setMessage(t("vod-captions-error-language"));
      return;
    }
    setBusy(true);
    setMessage("");
    try {
      await agent.client.call(place.stream.caption.importCaptions, {
        video,
        language: language.trim(),
        format: file.name.toLowerCase().endsWith(".srt") ? "srt" : "vtt",
        body: await file.text(),
      });
      setReload((value) => value + 1);
      setMessage(t("vod-captions-uploaded"));
    } catch (error) {
      console.error(error);
      setMessage(t("vod-captions-error-upload"));
    } finally {
      setBusy(false);
    }
  };
  return (
    <section
      data-testid="vod-captions"
      className="border-border mt-6 space-y-4 rounded-lg border p-4"
    >
      <h2 className="font-semibold">{t("vod-captions-title")}</h2>
      {tracks.length === 0 && (
        <p className="text-muted-foreground text-sm">
          {t("vod-captions-empty")}
        </p>
      )}
      {tracks.map((track) => (
        <div key={track.id} className="flex items-center gap-4">
          <span className="flex-1">
            {captionLanguageName(track.language, i18n.language)}
          </span>
          {(["vtt", "srt"] as const).map((format) => (
            <a
              key={format}
              data-testid={`vod-captions-download-${track.id}-${format}`}
              href={captionsUrl(url, { video }, track.id, format)}
              download
              className="focus-visible:ring-ring border-border rounded-md border px-3 py-2 text-sm focus-visible:ring-2"
            >
              {format.toUpperCase()}
            </a>
          ))}
        </div>
      ))}
      <label className="grid gap-2 text-sm">
        {t("vod-captions-language")}
        <Input
          data-testid="vod-captions-language"
          value={language}
          onChange={(event) => setLanguage(event.target.value)}
        />
      </label>
      <label className="grid gap-2 text-sm">
        {t("vod-captions-upload-description")}
        <input
          data-testid="vod-captions-upload"
          type="file"
          accept=".vtt,.srt,text/vtt,application/x-subrip"
          disabled={busy || !agent}
          onChange={(event) => {
            const file = event.target.files?.[0];
            event.target.value = "";
            if (file) void upload(file);
          }}
        />
      </label>
      {message && (
        <p role="status" data-testid="vod-captions-message">
          {message}
        </p>
      )}
    </section>
  );
}
