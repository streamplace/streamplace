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
} from "lucide-react-native";
import { useEffect, useState } from "react";
import { Text, View } from "react-native";
import * as zero from "../../ui";

interface InformationWidgetProps {
  embedMode?: boolean;
  wideMode?: boolean;
  isLive: boolean;
  viewers?: number;
  uptime?: string;
  connectionStatus?: "good" | "warning" | "error" | "neutral";
  timeBetweenSegments?: number;
  bitrate?: string;
  resolution?: string;
  fps?: string;
  videoCodec?: string;
  audioCodec?: string;
  audioChannels?: string;
  sampleRate?: string;
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
  isLive,
  viewers = 0,
  uptime = "00:00:00",
  connectionStatus = "neutral",
  timeBetweenSegments = 0,
  bitrate = "0 kbps",
  resolution = "Unknown",
  fps = "Unknown",
  videoCodec = "Unknown",
  audioCodec = "Unknown",
  audioChannels = "Unknown",
  sampleRate = "Unknown",
}: InformationWidgetProps) {
  const [currentTime, setCurrentTime] = useState(new Date());

  // Update time every second
  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentTime(new Date());
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  // Get bitrate status based on value
  const getBitrateStatus = (): "good" | "warning" | "error" | "neutral" => {
    const numericBitrate = parseInt(bitrate);
    if (numericBitrate > 2000) return "good";
    if (numericBitrate > 1000) return "warning";
    if (numericBitrate > 0) return "error";
    return "neutral";
  };

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
                    value={uptime}
                    status={isLive ? "good" : "neutral"}
                  />
                  <InfoRow
                    icon={Users}
                    label="Viewers"
                    value={isLive ? viewers.toLocaleString() : "0"}
                    status={isLive && viewers > 0 ? "good" : "neutral"}
                  />
                </Section>
              </View>

              <View style={[flex.values[1]]}>
                <Section title="Stream Health">
                  <InfoRow
                    icon={Wifi}
                    label="Connection"
                    value={connectionStatus}
                    status={connectionStatus}
                  />
                  <InfoRow
                    icon={Activity}
                    label="Segment Timing"
                    value={`${timeBetweenSegments}ms`}
                    status={connectionStatus}
                    subtext={
                      timeBetweenSegments > 5000 ? "High latency" : undefined
                    }
                  />
                  <InfoRow
                    icon={Zap}
                    label="Bitrate"
                    value={bitrate}
                    status={getBitrateStatus()}
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
                    value={resolution}
                    subtext={fps}
                  />
                  <InfoRow
                    icon={Video}
                    label="Video Codec"
                    value={videoCodec}
                  />
                </Section>
              </View>

              <View style={[flex.values[1]]}>
                <Section title="Audio">
                  <InfoRow
                    icon={Volume2}
                    label="Audio Codec"
                    value={audioCodec}
                  />
                  <InfoRow
                    icon={Radio}
                    label="Channels"
                    value={audioChannels}
                  />
                  <InfoRow
                    icon={Activity}
                    label="Sample Rate"
                    value={sampleRate}
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
