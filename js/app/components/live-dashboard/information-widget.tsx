import {
  useLivestreamStore,
  usePlayerStore,
  useProfile,
  useSegment,
  zero,
} from "@streamplace/components";
import {
  Activity,
  Clock,
  Monitor,
  Radio,
  Signal,
  Users,
  Video,
  Volume2,
  Wifi,
  Zap,
} from "@tamagui/lucide-icons";
import { useEffect, useState } from "react";
import { Text, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";
import { useSegmentTiming } from "../../hooks/useSegmentTiming";

interface InformationWidgetProps {
  embedMode?: boolean;
  wideMode?: boolean;
}

const { bg, r, borders, px, py, text, layout, gap, flex } = zero;

interface InfoRowProps {
  icon: any;
  label: string;
  value: string;
  status?: "good" | "warning" | "error" | "neutral";
  subtext?: string;
}

function InfoRow({
  icon: Icon,
  label,
  value,
  status = "neutral",
  subtext,
}: InfoRowProps) {
  const statusColors = {
    good: text.green[400],
    warning: text.yellow[400],
    error: text.red[400],
    neutral: text.white,
  };

  const statusColor = statusColors[status];

  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        py[2],
      ]}
    >
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        <Icon size={16} color="#9ca3af" />
        <Text style={[text.gray[300], { fontSize: 13, fontWeight: "500" }]}>
          {label}
        </Text>
      </View>
      <View style={[layout.flex.column, layout.flex.align.end]}>
        <Text style={[statusColor, { fontSize: 13, fontWeight: "600" }]}>
          {value}
        </Text>
        {subtext && (
          <Text style={[text.gray[500], { fontSize: 11 }]}>{subtext}</Text>
        )}
      </View>
    </View>
  );
}

interface SectionProps {
  title: string;
  children: React.ReactNode;
}

function Section({ title, children }: SectionProps) {
  return (
    <View style={[gap.all[2]]}>
      <Text
        style={[
          text.gray[200],
          { fontSize: 14, fontWeight: "700", letterSpacing: 0.5 },
        ]}
      >
        {title.toUpperCase()}
      </Text>
      <View
        style={[
          borders.color.neutral[700],
          borders.width.thin,
          { borderBottomWidth: 1 },
        ]}
      />
      <View style={[gap.all[1]]}>{children}</View>
    </View>
  );
}

export default function InformationWidget({
  embedMode = false,
  wideMode = false,
}: InformationWidgetProps = {}) {
  const [currentTime, setCurrentTime] = useState(new Date());

  // Hooks for data
  const userProfile = useProfile();
  const isLive = useLiveUser();
  const viewers = useLivestreamStore((x) => x.viewers);
  const segmentTiming = useSegmentTiming();
  const seg = useSegment();
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  const ingestStarted = usePlayerStore((x) => x.ingestStarted);

  // Update time every second
  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentTime(new Date());
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  // Calculate uptime
  const getUptime = (): string => {
    if (!ingestStarted || !isLive) return "00:00:00";
    const uptimeMs = Date.now() - ingestStarted;
    const seconds = Math.floor(uptimeMs / 1000);
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    return `${hours.toString().padStart(2, "0")}:${minutes.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  // Calculate bitrate
  const getBitrate = (): {
    value: string;
    status: "good" | "warning" | "error" | "neutral";
  } => {
    if (!seg?.size || !seg?.duration) {
      return { value: "0 kbps", status: "neutral" };
    }

    const kbps =
      (seg.size * 8) /
      ((seg.duration || 1000000000) / 1000000000) /
      1000 /
      1000;

    // Determine status based on bitrate
    let status: "good" | "warning" | "error" | "neutral" = "neutral";
    if (kbps > 2000) status = "good";
    else if (kbps > 1000) status = "warning";
    else if (kbps > 0) status = "error";

    return {
      value: `${kbps.toFixed(0)} kbps`,
      status,
    };
  };

  // Get connection quality status
  const getConnectionStatus = (): "good" | "warning" | "error" | "neutral" => {
    if (!isLive) return "neutral";
    switch (segmentTiming.connectionQuality) {
      case "good":
        return "good";
      case "degraded":
        return "warning";
      case "poor":
        return "error";
      default:
        return "neutral";
    }
  };

  // Get stream status
  const getStreamStatus = (): "live" | "offline" | "starting" | "error" => {
    if (!isLive) return "offline";
    if (ingestConnectionState === "connecting") return "starting";
    if (segmentTiming.connectionQuality === "poor") return "error";
    return "live";
  };

  const bitrate = getBitrate();
  const streamStatus = getStreamStatus();

  // Extract video and audio information from segment
  const getMediaInfo = () => {
    const video = seg?.video?.[0];
    const audio = seg?.audio?.[0];

    if (!video && !audio) {
      return {
        resolution: "Unknown",
        fps: "Unknown",
        codec: "Unknown",
        profile: "Unknown",
        audioCodec: "Unknown",
        audioChannels: "Unknown",
        sampleRate: "Unknown",
      };
    }

    // Video info
    const resolution =
      video?.width && video?.height
        ? `${video.width}x${video.height}`
        : "Unknown";

    // Handle fractional framerate (num/den format)
    let fps = "Unknown";
    if (video?.framerate?.num && video?.framerate?.den) {
      const fpsValue = video.framerate.num / video.framerate.den;
      fps = `${fpsValue.toFixed(2)} FPS`;
    } else if (video?.frameRate || video?.fps) {
      const fpsValue = video.frameRate || video.fps;
      fps = `${fpsValue} FPS`;
    }

    const codec = video?.codec ? video.codec.toUpperCase() : "Unknown";
    const profile = video?.profile || "";

    // Audio info
    const audioCodec = audio?.codec ? audio.codec.toUpperCase() : "Unknown";
    const audioChannels = audio?.channels ? `${audio.channels} ch` : "Unknown";
    const sampleRate = audio?.rate
      ? `${(audio.rate / 1000).toFixed(1)}kHz`
      : "Unknown";

    return {
      resolution,
      fps,
      codec,
      profile,
      audioCodec,
      audioChannels,
      sampleRate,
    };
  };

  const mediaInfo = getMediaInfo();

  return (
    <View
      style={[
        embedMode
          ? { backgroundColor: "rgba(23, 23, 23, 0.9)" }
          : bg.neutral[900],
        embedMode ? undefined : borders.width.thin,
        embedMode ? undefined : borders.color.neutral[700],
        r.lg,
        px[4],
        py[4],
        gap.all[6],
        {
          minWidth: wideMode ? 400 : 220,
          width: wideMode ? "100%" : undefined,
        },
      ]}
    >
      <View
        style={[wideMode ? layout.flex.row : layout.flex.column, gap.all[1]]}
      >
        <Text
          style={[text.white, { fontSize: 18, fontWeight: "700" }]}
          numberOfLines={1}
        >
          Stream Information
        </Text>
      </View>
      <View
        style={[
          {
            minWidth: wideMode ? 400 : 320,
            width: wideMode ? "100%" : undefined,
          },
        ]}
      >
        {wideMode ? (
          <View style={[flex.values[1]]}>
            {/* Top Row */}
            <View style={[layout.flex.row, gap.all[6]]}>
              <View style={[flex.values[1]]}>
                <Section title="Stream Info">
                  <InfoRow
                    icon={Clock}
                    label="Current Time"
                    value={currentTime.toLocaleTimeString()}
                  />
                  <InfoRow
                    icon={Signal}
                    label="Uptime"
                    value={getUptime()}
                    status={isLive ? "good" : "neutral"}
                  />
                  <InfoRow
                    icon={Users}
                    label="Viewers"
                    value={isLive ? (viewers || 0).toLocaleString() : "0"}
                    status={
                      isLive && viewers && viewers > 0 ? "good" : "neutral"
                    }
                  />
                </Section>
              </View>

              <View style={[flex.values[1]]}>
                <Section title="Stream Health">
                  <InfoRow
                    icon={Wifi}
                    label="Connection"
                    value={segmentTiming.connectionQuality || "Unknown"}
                    status={getConnectionStatus()}
                  />
                  <InfoRow
                    icon={Activity}
                    label="Segment Timing"
                    value={
                      segmentTiming.timeBetweenSegments
                        ? `${segmentTiming.timeBetweenSegments}ms`
                        : "0ms"
                    }
                    status={getConnectionStatus()}
                    subtext={
                      segmentTiming.timeBetweenSegments &&
                      segmentTiming.timeBetweenSegments > 5000
                        ? "High latency"
                        : undefined
                    }
                  />
                  <InfoRow
                    icon={Zap}
                    label="Bitrate"
                    value={bitrate.value}
                    status={bitrate.status}
                  />
                </Section>
              </View>
            </View>
            {/* Bottom Row */}
            <View style={[layout.flex.row, gap.all[6]]}>
              <View style={[flex.values[1]]}>
                <Section title="Video">
                  <InfoRow
                    icon={Monitor}
                    label="Resolution"
                    value={mediaInfo.resolution}
                    subtext={mediaInfo.fps}
                  />
                  <InfoRow
                    icon={Video}
                    label="Video Codec"
                    value={mediaInfo.codec}
                  />
                </Section>
              </View>

              <View style={[flex.values[1]]}>
                <Section title="Audio">
                  <InfoRow
                    icon={Volume2}
                    label="Audio Codec"
                    value={mediaInfo.audioCodec}
                  />
                  <InfoRow
                    icon={Radio}
                    label="Channels"
                    value={mediaInfo.audioChannels}
                  />
                  <InfoRow
                    icon={Activity}
                    label="Sample Rate"
                    value={mediaInfo.sampleRate}
                  />
                </Section>
              </View>
            </View>
          </View>
        ) : null}
      </View>
    </View>
  );
}
