import {
  Button,
  Input,
  MenuContainer,
  MenuGroup,
  MenuItem,
  MenuLabel,
  SegmentedTabs,
  Select,
  Text,
  useTheme,
  useToast,
  View,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { SettingToggle } from "components/settings/components/setting-toggle";
import { useEffect, useState } from "react";
import { place } from "streamplace";

const VIDEO = "place.stream.ingest.videoRendition";
const AUDIO = "place.stream.ingest.audioRendition";
type Collection = typeof VIDEO | typeof AUDIO;

// Video sizes as "WxH" and frame rates as "numerator/denominator", so a saved
// value that isn't one of these (60000/1001, say) survives a save.
const VIDEO_CODECS = { h264: "H.264", av1: "AV1" } as const;
const SIZES = ["1280x720", "1920x1080", "2560x1440", "3840x2160"];
const FRAMERATES = ["30/1", "60/1", "120/1"];
const withLoaded = (options: string[], loaded: string) =>
  options.includes(loaded) ? options : [...options, loaded];
const fpsLabel = (rate: string) => {
  const [n, d] = rate.split("/").map(Number);
  return `${Number.isInteger(n / d) ? n / d : (n / d).toFixed(2)} fps`;
};

interface Video {
  codec: string;
  width: number;
  height: number;
  framerate: { numerator: number; denominator: number };
  bitrate: number;
}

// With AV1, OBS also encodes an H.264 copy for devices that can't decode AV1;
// the node takes the first video record as the source, so it's written after
// (rkeys sort in order). At most 1080p, at the AV1 rate halved until it's at
// most 60 fps (so it still divides OBS's), at 6000 kbps for 1080p60 scaled by
// its pixel rate.
const h264Fallback = ({ width, height, framerate }: Video): Video => {
  const scale = Math.min(1, 1080 / height);
  const w = 2 * Math.round((width * scale) / 2);
  const h = 2 * Math.round((height * scale) / 2);
  let { numerator, denominator } = framerate;
  while (numerator / denominator > 60) {
    if (numerator % 2 === 0) numerator /= 2;
    else denominator *= 2;
  }
  return {
    codec: "h264",
    width: w,
    height: h,
    framerate: { numerator, denominator },
    bitrate: Math.max(
      1,
      100 * Math.round((w * h * numerator) / (1920 * 1080 * denominator)),
    ),
  };
};

const AUDIO_BITRATES = { aac: 160, opus: 128 } as const;
type AudioCodec = keyof typeof AUDIO_BITRATES;

interface Settings {
  video: boolean;
  codec: string;
  size: string;
  fps: string;
  bitrate: string;
  aac: boolean;
  opus: boolean;
}

const DEFAULTS: Settings = {
  video: false,
  codec: "h264",
  size: "1920x1080",
  fps: "60/1",
  bitrate: "6000",
  aac: false,
  opus: false,
};

// The records the form stands for: saving writes these, and deletes whatever
// else it found, so it never leaves a record it doesn't show. Audio keeps
// each record's bitrate, which the form doesn't edit.
interface Keys {
  video: string[];
  audio: { rkey: string; codec: string; bitrate: number }[];
}

/**
 * The renditions OBS encodes when it streams to this node, kept on the node
 * as place.stream.ingest.* records (place.stream.local.*). With none of a
 * kind, OBS uses its own encoder for it.
 */
export function RenditionSettings() {
  const { theme } = useTheme();
  const toast = useToast();
  const agent = usePDSAgent();
  const did = agent?.did as `did:${string}:${string}` | undefined;
  const [settings, setSettings] = useState<Settings | null>(null);
  const [keys, setKeys] = useState<Keys>({ video: [], audio: [] });
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!agent || !did) return;
    (async () => {
      try {
        const list = (collection: Collection) =>
          agent.client.call(place.stream.local.listRecords, {
            repo: did,
            collection,
          });
        const [video, audio] = await Promise.all([list(VIDEO), list(AUDIO)]);
        const rkey = (uri: string) => uri.split("/").pop()!;
        const v = video.records[0]?.value as any;
        const loaded: Keys = {
          video: video.records.map((r) => rkey(r.uri)),
          audio: audio.records.map((r) => ({
            rkey: rkey(r.uri),
            codec: (r.value as any).codec,
            bitrate: (r.value as any).bitrate,
          })),
        };
        setKeys(loaded);
        setSettings({
          ...DEFAULTS,
          ...(v && {
            video: true,
            codec: v.codec,
            size: `${v.width}x${v.height}`,
            fps: `${v.framerate.numerator}/${v.framerate.denominator}`,
            bitrate: String(v.bitrate),
          }),
          aac: loaded.audio.some((a) => a.codec === "aac"),
          opus: loaded.audio.some((a) => a.codec === "opus"),
        });
      } catch (e: any) {
        toast.show("Couldn't load renditions", e.message, { variant: "error" });
      }
    })();
  }, [agent, did]);

  if (!settings || !agent || !did) {
    return null;
  }
  const set = (patch: Partial<Settings>) =>
    setSettings({ ...settings, ...patch });
  const bitrate = parseInt(settings.bitrate, 10);
  const valid = !settings.video || bitrate > 0;
  const [width, height] = settings.size.split("x").map(Number);
  const [numerator, denominator] = settings.fps.split("/").map(Number);
  const source: Video = {
    codec: settings.codec,
    width,
    height,
    framerate: { numerator, denominator },
    bitrate,
  };
  const fallback = settings.codec === "av1" ? h264Fallback(source) : null;
  const main = keys.video[0] ?? "main";
  const video: [string, Video][] = !settings.video
    ? []
    : fallback
      ? [
          [main, source],
          [`${main}-h264`, fallback],
        ]
      : [[main, source]];

  const save = async () => {
    setSaving(true);
    const createdAt = new Date().toISOString();
    const put = (collection: Collection, rkey: string, record: object) =>
      agent.client.call(place.stream.local.putRecord, {
        repo: did,
        collection,
        rkey,
        record: { $type: collection, ...record, createdAt },
      });
    const del = (collection: Collection, rkey: string) =>
      agent.client.call(place.stream.local.deleteRecord, {
        repo: did,
        collection,
        rkey,
      });
    // A new record's key is its codec, or the first free variant of it: a
    // loaded record may hold another codec under that name.
    const taken = new Set(keys.audio.map((a) => a.rkey));
    const freeKey = (codec: string) => {
      let key = codec;
      for (let n = 2; taken.has(key); n++) key = `${codec}-${n}`;
      taken.add(key);
      return key;
    };
    const next: Keys = {
      video: video.map(([rkey]) => rkey),
      audio: (Object.keys(AUDIO_BITRATES) as AudioCodec[])
        .filter((codec) => settings[codec])
        .map(
          (codec) =>
            keys.audio.find((a) => a.codec === codec) ?? {
              codec,
              rkey: freeKey(codec),
              bitrate: AUDIO_BITRATES[codec],
            },
        ),
    };
    // What the node holds, updated as each write lands, so the keys still
    // match it when a save fails partway.
    const held: Keys = { video: [...keys.video], audio: [...keys.audio] };
    try {
      for (const rkey of keys.video.filter((k) => !next.video.includes(k))) {
        await del(VIDEO, rkey);
        held.video = held.video.filter((k) => k !== rkey);
      }
      for (const a of keys.audio.filter(
        (a) => !next.audio.some((n) => n.rkey === a.rkey),
      )) {
        await del(AUDIO, a.rkey);
        held.audio = held.audio.filter((h) => h !== a);
      }
      for (const [rkey, record] of video) {
        await put(VIDEO, rkey, record);
        if (!held.video.includes(rkey)) held.video.push(rkey);
      }
      for (const a of next.audio) {
        await put(AUDIO, a.rkey, { codec: a.codec, bitrate: a.bitrate });
        held.audio = [...held.audio.filter((h) => h.rkey !== a.rkey), a];
      }
      toast.show("Saved", "OBS picks these up the next time it goes live.");
    } catch (e: any) {
      toast.show("Couldn't save renditions", e.message, { variant: "error" });
    } finally {
      setKeys(held);
      setSaving(false);
    }
  };

  return (
    <MenuContainer>
      <MenuLabel>Video</MenuLabel>
      <MenuGroup>
        <SettingToggle
          title="Encode video here"
          description="Off: OBS's own video settings."
          value={settings.video}
          onValueChange={(video) => set({ video })}
          testID="desktop-video"
        />
        {settings.video && (
          <MenuItem>
            <View style={{ flex: 1, gap: theme.spacing[3] }}>
              <SegmentedTabs
                options={withLoaded(
                  Object.keys(VIDEO_CODECS),
                  settings.codec,
                ).map((codec) => ({
                  label:
                    VIDEO_CODECS[codec as keyof typeof VIDEO_CODECS] ?? codec,
                  value: codec,
                }))}
                value={settings.codec}
                onChange={(codec) => set({ codec })}
              />
              <Select
                value={settings.size}
                onValueChange={(size) => set({ size })}
                items={withLoaded(SIZES, settings.size).map((size) => ({
                  label: `${size.split("x")[1]}p`,
                  value: size,
                }))}
              />
              <SegmentedTabs
                options={withLoaded(FRAMERATES, settings.fps).map((fps) => ({
                  label: fpsLabel(fps),
                  value: fps,
                }))}
                value={settings.fps}
                onChange={(fps) => set({ fps })}
              />
              <Input
                label="Bitrate (kbps)"
                value={settings.bitrate}
                onChangeText={(bitrate) => set({ bitrate })}
                keyboardType="numeric"
                error={valid ? undefined : "Enter a bitrate in kbps"}
                testID="desktop-video-bitrate"
              />
              {fallback && (
                <Text size="sm" color="muted">
                  {`Also H.264 ${fallback.height}p ${fpsLabel(`${fallback.framerate.numerator}/${fallback.framerate.denominator}`)} at ${fallback.bitrate} kbps, for devices that can't play AV1.`}
                </Text>
              )}
            </View>
          </MenuItem>
        )}
      </MenuGroup>
      <MenuLabel>Audio</MenuLabel>
      <MenuGroup>
        <SettingToggle
          title="AAC"
          description="For HLS, VODs and multistreaming."
          value={settings.aac}
          onValueChange={(aac) => set({ aac })}
          testID="desktop-audio-aac"
        />
        <SettingToggle
          title="Opus"
          description="For WebRTC. With neither, OBS's own audio settings."
          value={settings.opus}
          onValueChange={(opus) => set({ opus })}
          testID="desktop-audio-opus"
        />
      </MenuGroup>
      <View style={{ paddingTop: theme.spacing[4] }}>
        <Button
          onPress={save}
          disabled={!valid || saving}
          loading={saving}
          testID="desktop-save"
        >
          Save
        </Button>
      </View>
    </MenuContainer>
  );
}
