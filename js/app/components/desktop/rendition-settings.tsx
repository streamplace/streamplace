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

// Video sizes by height. One video track for now; the records are a list so
// a second encode (AV1) is another record, not a new shape.
const SIZES: Record<string, { width: number; height: number }> = {
  "720": { width: 1280, height: 720 },
  "1080": { width: 1920, height: 1080 },
  "1440": { width: 2560, height: 1440 },
  "2160": { width: 3840, height: 2160 },
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
  size: "1080",
  fps: "60",
  bitrate: "6000",
  aac: false,
  opus: false,
};

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
        const v = video.records[0]?.value as any;
        const codecs = audio.records.map((r) => (r.value as any).codec);
        setSettings({
          ...DEFAULTS,
          ...(v && {
            video: true,
            size: String(v.height),
            fps: String(v.framerate.numerator / v.framerate.denominator),
            bitrate: String(v.bitrate),
          }),
          aac: codecs.includes("aac"),
          opus: codecs.includes("opus"),
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
    try {
      await (settings.video
        ? put(VIDEO, "main", {
            codec: "h264",
            ...SIZES[settings.size],
            framerate: {
              numerator: parseInt(settings.fps, 10),
              denominator: 1,
            },
            bitrate,
          })
        : del(VIDEO, "main"));
      for (const codec of Object.keys(AUDIO_BITRATES) as AudioCodec[]) {
        await (settings[codec]
          ? put(AUDIO, codec, { codec, bitrate: AUDIO_BITRATES[codec] })
          : del(AUDIO, codec));
      }
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
                items={Object.keys(SIZES).map((h) => ({
                  label: `${h}p`,
                  value: h,
                }))}
              />
              <SegmentedTabs
                options={["30", "60"].map((fps) => ({
                  label: `${fps} fps`,
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
