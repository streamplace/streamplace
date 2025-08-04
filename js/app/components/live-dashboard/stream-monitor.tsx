import {
  Player,
  useLivestreamStore,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import { Camera, Signal, Wifi, WifiOff } from "@tamagui/lucide-icons";
import { Text, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";
import { useSegmentTiming } from "../../hooks/useSegmentTiming";

const { flex, bg, r, borders, layout, p, text, w, h } = zero;

interface StreamMonitorProps {
  userProfile?: any;
  isLive?: boolean;
  videoRef?: any;
}

export default function StreamMonitor({
  userProfile: propUserProfile,
  isLive: propIsLive,
  videoRef,
}: StreamMonitorProps) {
  // Get data from hooks - use props as fallback if provided
  const isUserLive = useLiveUser();
  const profile = useLivestreamStore((x) => x.profile);
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  const segmentTiming = useSegmentTiming();

  // Use hook data primarily, fallback to props
  const isLive = propIsLive ?? isUserLive;
  const userProfile = propUserProfile ?? profile;

  // Connection quality indicator
  const getConnectionIcon = () => {
    if (!isLive) return null;

    switch (segmentTiming.connectionQuality) {
      case "good":
        return <Wifi size={16} color="#10b981" />;
      case "degraded":
        return <Signal size={16} color="#f59e0b" />;
      case "poor":
        return <WifiOff size={16} color="#ef4444" />;
      default:
        return <WifiOff size={16} color="#6b7280" />;
    }
  };

  const getConnectionColor = () => {
    if (!isLive) return "red";

    switch (segmentTiming.connectionQuality) {
      case "good":
        return "green";
      case "degraded":
        return "yellow";
      case "poor":
        return "red";
      default:
        return "red";
    }
  };
  return (
    <View
      style={[
        flex.values[2],
        bg.gray[800],
        r[3],
        borders.width.thin,
        borders.color.gray[700],
        layout.flex.column,
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          p[4],
          borders.bottom.width.thin,
          borders.bottom.color.gray[700],
        ]}
      >
        <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
          Stream Monitor
        </Text>
        <View style={[layout.flex.row, layout.flex.alignCenter, { gap: 8 }]}>
          {getConnectionIcon()}
          <View style={[w[2], h[2], r[1], bg[getConnectionColor()][500]]} />
          <Text style={[text.gray[400], { fontSize: 14 }]}>
            {isLive ? "LIVE" : "OFFLINE"}
          </Text>
          {isLive && segmentTiming.timeBetweenSegments && (
            <Text style={[text.gray[500], { fontSize: 12 }]}>
              {Math.round(segmentTiming.timeBetweenSegments)}ms
            </Text>
          )}
        </View>
      </View>

      <View style={[flex.values[1], layout.flex.center, bg.gray[900]]}>
        {isLive && userProfile ? (
          <Player src={userProfile.did} name={userProfile.handle} />
        ) : (
          <View style={[layout.flex.center, { gap: 12 }]}>
            <Camera size={48} color="#6b7280" />
            <Text style={[text.gray[400]]}>
              {!userProfile ? "No Profile" : "Stream Offline"}
            </Text>
            {ingestConnectionState && (
              <Text style={[text.gray[500], { fontSize: 12 }]}>
                Connection: {ingestConnectionState}
              </Text>
            )}
          </View>
        )}
      </View>
    </View>
  );
}
