import { isValidAtUri } from "@atproto/syntax";
import {
  captionLanguageName,
  captionsUrl,
  CaptionTrackView,
  fetchCaptionTracks,
} from "@streamplace/core";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Linking, Platform, View } from "react-native";
import { place } from "streamplace";
import { useTheme } from "../../lib/theme/theme";
import { useStreamplaceStore } from "../../streamplace-store";
import { usePDSAgent } from "../../streamplace-store/xrpc";
import { Button } from "../ui/button";
import { Input } from "../ui/input";
import { Text } from "../ui/text";

// BCP-47-ish: a 2-3 letter primary tag plus optional subtags.
const LANGUAGE_TAG = /^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$/;

/** Asks the browser for a .vtt/.srt file. Web only. */
function pickCaptionFile(): Promise<File | null> {
  const { promise, resolve } = Promise.withResolvers<File | null>();
  const input = document.createElement("input");
  input.type = "file";
  input.accept = ".vtt,.srt,text/vtt,application/x-subrip";
  input.onchange = () => resolve(input.files?.[0] ?? null);
  input.click();
  return promise;
}

/**
 * Caption tracks of one of the viewer's videos: lists them with VTT/SRT
 * downloads, and imports a .vtt or .srt file per language
 * (place.stream.caption.importCaptions). Uploading needs a file picker,
 * so it is offered on the web only.
 */
export function VideoCaptionsManager({ video }: { video: string }) {
  const { t, i18n } = useTranslation();
  const { theme } = useTheme();
  const url = useStreamplaceStore((x) => x.url);
  const agent = usePDSAgent();
  const [tracks, setTracks] = useState<CaptionTrackView[]>([]);
  const [reload, setReload] = useState(0);
  const [language, setLanguage] = useState("");
  const [uploading, setUploading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    fetchCaptionTracks(url, { video }, controller.signal)
      .then(setTracks)
      .catch((err) => {
        if (!controller.signal.aborted) {
          console.error("failed to list caption tracks", err);
        }
      });
    return () => controller.abort();
  }, [url, video, reload]);

  const upload = async () => {
    if (!LANGUAGE_TAG.test(language.trim())) {
      setMessage(t("vod-captions-error-language"));
      return;
    }
    if (!agent || !isValidAtUri(video)) return;
    const file = await pickCaptionFile();
    if (!file) return;
    setUploading(true);
    setMessage(null);
    try {
      await agent.client.call(place.stream.caption.importCaptions, {
        video,
        language: language.trim(),
        format: file.name.toLowerCase().endsWith(".srt") ? "srt" : "vtt",
        body: await file.text(),
      });
      setMessage(t("vod-captions-uploaded"));
      setReload((n) => n + 1);
    } catch (err) {
      console.error("failed to import captions", err);
      setMessage(t("vod-captions-error-upload"));
    } finally {
      setUploading(false);
    }
  };

  const download = (track: string, format: "vtt" | "srt") => {
    const href = captionsUrl(url, { video }, track, format);
    if (Platform.OS === "web") window.open(href, "_blank");
    else Linking.openURL(href);
  };

  return (
    <View testID="vod-captions" style={{ gap: theme.spacing[3] }}>
      <Text weight="semibold">{t("vod-captions-title")}</Text>
      {tracks.length === 0 ? (
        <Text size="sm" muted>
          {t("vod-captions-empty")}
        </Text>
      ) : (
        tracks.map((track) => {
          const name = captionLanguageName(track.language, i18n.language);
          return (
            <View
              key={track.id}
              testID={`vod-captions-track-${track.id}`}
              style={{
                flexDirection: "row",
                alignItems: "center",
                gap: theme.spacing[2],
              }}
            >
              <Text style={{ flex: 1 }}>
                {track.source === "auto"
                  ? t("player-captions-track-auto", { language: name })
                  : name}
              </Text>
              <Button
                variant="ghost"
                size="sm"
                width="min"
                testID={`vod-captions-download-${track.id}-vtt`}
                onPress={() => download(track.id, "vtt")}
              >
                VTT
              </Button>
              <Button
                variant="ghost"
                size="sm"
                width="min"
                testID={`vod-captions-download-${track.id}-srt`}
                onPress={() => download(track.id, "srt")}
              >
                SRT
              </Button>
            </View>
          );
        })
      )}
      {Platform.OS === "web" && (
        <View style={{ gap: theme.spacing[2] }}>
          <Text size="sm" muted>
            {t("vod-captions-upload-description")}
          </Text>
          <View
            style={{
              flexDirection: "row",
              alignItems: "center",
              gap: theme.spacing[2],
            }}
          >
            <View style={{ flex: 1 }}>
              <Input
                testID="vod-captions-language"
                value={language}
                onChangeText={setLanguage}
                placeholder={t("vod-captions-language-placeholder")}
                accessibilityLabel={t("vod-captions-language")}
                autoCapitalize="none"
              />
            </View>
            <Button
              variant="secondary"
              width="min"
              testID="vod-captions-upload"
              disabled={uploading || !agent}
              loading={uploading}
              onPress={upload}
            >
              {t("vod-captions-upload")}
            </Button>
          </View>
          {message && (
            <Text size="sm" muted testID="vod-captions-message">
              {message}
            </Text>
          )}
        </View>
      )}
    </View>
  );
}
