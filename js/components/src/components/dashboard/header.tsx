import { Image, Pressable, Text, View } from "react-native";
import {
  borderAlphas,
  fontFamilies,
  surfaces,
  textAlphas,
} from "../../lib/theme/tokens";
import * as zero from "../../ui";
import { Badge, LiveBadge } from "../ui/badge";

const { r, borders, px, py, text, layout, gap } = zero;

// A single console readout — uppercase micro-label over a GeistMono value.
function MetaReadout({ label, value }: { label: string; value: string }) {
  return (
    <View style={{ gap: 2 }}>
      <Text
        style={{
          color: textAlphas.dark[3],
          fontSize: 11,
          fontWeight: "500",
          letterSpacing: 0.5,
          textTransform: "uppercase",
        }}
      >
        {label}
      </Text>
      <Text
        style={[
          text.white,
          { fontSize: 14, fontFamily: fontFamilies.monoMedium },
        ]}
      >
        {value}
      </Text>
    </View>
  );
}

interface HeaderProps {
  isLive: boolean;
  streamTitle?: string;
  uptime?: string;
  bitrate?: string;
  timeBetweenSegments?: number;
  connectionStatus?: "excellent" | "good" | "poor" | "offline" | "pre-live";
  problemsCount?: number;
  onProblemsPress?: () => void;
  avatarUrl?: string;
}

export default function Header({
  isLive,
  streamTitle = "Live Stream",
  uptime = "00:00:00",
  bitrate = "0 mbps",
  connectionStatus,
  problemsCount = 0,
  onProblemsPress,
  avatarUrl,
}: HeaderProps) {
  // Three distinct states, so the badge never contradicts the preview/health:
  //   offline  — no ingest
  //   preview  — segments flowing but not published ("Start Livestream!" unpressed)
  //   live     — published to the network (red, reserved)
  const isPreview = isLive && connectionStatus === "pre-live";
  const isTrulyLive = isLive && !isPreview;

  // Uptime is only meaningful once we have a real start; OBS ingest never sets
  // the app's ingest clock, so it reads a dead "00:00:00" — drop it then.
  const showUptime = isTrulyLive && uptime !== "00:00:00";
  // Bitrate confirms signal in both preview and live.
  const showBitrate = isLive && bitrate !== "0 mbps";

  return (
    <View
      style={[
        px[4],
        py[3],
        r.lg,
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
        { backgroundColor: surfaces.dark[1] },
        borders.width.thin,
        { borderColor: borderAlphas.dark.strong },
      ]}
    >
      {/* Identity + status */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        {avatarUrl ? (
          <Image
            source={{ uri: avatarUrl }}
            style={{
              width: 38,
              height: 38,
              borderRadius: 19,
              backgroundColor: surfaces.dark[2],
            }}
          />
        ) : null}
        <View style={{ gap: 5 }}>
          <Text
            style={[text.white, { fontSize: 15, fontWeight: "600" }]}
            numberOfLines={1}
          >
            {streamTitle}
          </Text>
          <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
            {isTrulyLive ? (
              <LiveBadge />
            ) : isPreview ? (
              <Badge variant="accent">PREVIEW</Badge>
            ) : (
              <Badge>OFFLINE</Badge>
            )}
            {problemsCount > 0 ? (
              <Pressable onPress={onProblemsPress}>
                <Badge variant="warning">
                  {problemsCount} {problemsCount === 1 ? "Issue" : "Issues"}
                </Badge>
              </Pressable>
            ) : null}
          </View>
        </View>
      </View>

      {/* Live meta — mono readouts. Bitrate shows in preview too (signal check);
          uptime only once truly live with a real clock. */}
      {showUptime || showBitrate ? (
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[6]]}>
          {showUptime ? <MetaReadout label="Uptime" value={uptime} /> : null}
          {showBitrate ? <MetaReadout label="Bitrate" value={bitrate} /> : null}
        </View>
      ) : null}
    </View>
  );
}
