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
// value that isn't one of these (60000/1001, say) survives a save. One video
// track for now; the records are a list so a second encode (AV1) is another
// record, not a new shape.
const SIZES = ["1280x720", "1920x1080", "2560x1440", "3840x2160"];
const FRAMERATES = ["30/1", "60/1"];
const withLoaded = (options: string[], loaded: string) =>
  options.includes(loaded) ? options : [...options, loaded];
const fpsLabel = (rate: string) => {
  const [n, d] = rate.split("/").map(Number);
  return `${Number.isInteger(n / d) ? n / d : (n / d).toFixed(2)} fps`;
};

const AUDIO_BITRATES = { aac: 160, opus: 128 } as const;
type AudioCodec = keyof typeof AUDIO_BITRATES;

interface Settings {
  video: boolean;
  size: string;
  fps: string;
  bitrate: string;
  aac: boolean;
  opus: boolean;
}

const DEFAULTS: Settings = {
  video: false,
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
    const [numerator, denominator] = settings.fps.split("/").map(Number);
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
      video: settings.video ? [keys.video[0] ?? "main"] : [],
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
    try {
      for (const rkey of keys.video.filter((k) => !next.video.includes(k))) {
        await del(VIDEO, rkey);
      }
      for (const { rkey } of keys.audio.filter(
        (a) => !next.audio.some((n) => n.rkey === a.rkey),
      )) {
        await del(AUDIO, rkey);
      }
      for (const rkey of next.video) {
        await put(VIDEO, rkey, {
          codec: "h264",
          width: Number(settings.size.split("x")[0]),
          height: Number(settings.size.split("x")[1]),
          framerate: { numerator, denominator },
          bitrate,
        });
      }
      for (const { rkey, codec, bitrate } of next.audio) {
        await put(AUDIO, rkey, { codec, bitrate });
      }
      setKeys(next);
      toast.show("Saved", "OBS picks these up the next time it goes live.");
    } catch (e: any) {
      toast.show("Couldn't save renditions", e.message, { variant: "error" });
    } finally {
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
              <Text size="sm" color="muted">
                H.264
              </Text>
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
