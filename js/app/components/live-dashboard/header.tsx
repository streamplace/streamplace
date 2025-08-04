import {
  useLivestreamStore,
  usePlayerStore,
  useProfile,
  zero,
} from "@streamplace/components";
import {
  Activity,
  Monitor,
  Radio,
  Signal,
  Users,
  Wifi,
} from "@tamagui/lucide-icons";
import { Text, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";
import { useSegmentTiming } from "../../hooks/useSegmentTiming";

const { flex, bg, r, borders, p, px, py, text, layout, gap } = zero;

interface MetricItemProps {
  icon: any;
  label: string;
  value: string;
  status?: "good" | "warning" | "error";
}

function MetricItem({ icon: Icon, label, value, status }: MetricItemProps) {
  const statusColors = {
    good: text.green[400],
    warning: text.yellow[400],
    error: text.red[400],
  };

  const statusColor = status ? statusColors[status] : text.gray[300];

  return (
    <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
      <Icon size={16} color="#9ca3af" />
      <View style={[layout.flex.column]}>
        <Text style={[text.gray[400], { fontSize: 11, fontWeight: "500" }]}>
          {label}
        </Text>
        <Text style={[statusColor, { fontSize: 13, fontWeight: "600" }]}>
          {value}
        </Text>
      </View>
    </View>
  );
}

interface StatusIndicatorProps {
  status: "excellent" | "good" | "poor" | "offline";
  isLive: boolean;
}

function StatusIndicator({ status, isLive }: StatusIndicatorProps) {
  const getStatusColor = () => {
    if (!isLive) return bg.gray[500];
    switch (status) {
      case "excellent":
        return bg.green[500];
      case "good":
        return bg.yellow[500];
      case "poor":
        return bg.orange[500];
      case "offline":
        return bg.red[500];
      default:
        return bg.gray[500];
    }
  };

  const getStatusText = () => {
    if (!isLive) return "OFFLINE";
    switch (status) {
      case "excellent":
        return "EXCELLENT";
      case "good":
        return "GOOD";
      case "poor":
        return "POOR";
      case "offline":
        return "OFFLINE";
      default:
        return "UNKNOWN";
    }
  };

  return (
    <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
      <View
        style={[
          { width: 8, height: 8, borderRadius: 4 },
          getStatusColor(),
          !isLive && { opacity: 0.6 },
        ]}
      />
      <Text
        style={[
          text.white,
          { fontSize: 12, fontWeight: "700", letterSpacing: 0.5 },
          !isLive && text.gray[400],
        ]}
      >
        {getStatusText()}
      </Text>
    </View>
  );
}

interface HeaderProps {
  isLive?: boolean;
}

export default function Header({ isLive: propIsLive }: HeaderProps) {
  // Get real data from hooks
  const userProfile = useProfile();
  const isUserLive = useLiveUser();
  const viewers = useLivestreamStore((x) => x.viewers);
  const segmentTiming = useSegmentTiming();
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  const ingestStarted = usePlayerStore((x) => x.ingestStarted);

  // Use hook data primarily, fallback to props
  const isLive = propIsLive ?? isUserLive;

  // Calculate uptime from ingest start time
  const getUptime = (): string => {
    if (!ingestStarted || !isLive) return "00:00:00";
    const uptimeMs = Date.now() - ingestStarted;
    const seconds = Math.floor(uptimeMs / 1000);
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    return `${hours.toString().padStart(2, "0")}:${minutes.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  // Map connection quality to status
  const getConnectionStatus = (): "excellent" | "good" | "poor" | "offline" => {
    if (!isLive) return "offline";
    switch (segmentTiming.connectionQuality) {
      case "good":
        return "excellent";
      case "degraded":
        return "good";
      case "poor":
        return "poor";
      default:
        return "offline";
    }
  };

  const getFpsStatus = (fps: number): "good" | "warning" | "error" => {
    if (fps >= 30) return "good";
    if (fps >= 20) return "warning";
    return "error";
  };

  const getBitrateStatus = (bitrate: string): "good" | "warning" | "error" => {
    const value = parseInt(bitrate);
    if (value >= 2000) return "good";
    if (value >= 1000) return "warning";
    return "error";
  };

  return (
    <View
      style={[
        bg.gray[800],
        borders.bottom.width.thin,
        borders.bottom.color.gray[700],
        px[4],
        py[3],
        layout.flex.row,
        layout.flex.alignCenter,
        layout.flex.spaceBetween,
      ]}
    >
      {/* Left side - Stream title and status */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[4]]}>
        <View>
          <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
            {userProfile?.displayName || userProfile?.handle || "Live Stream"}
          </Text>
          <StatusIndicator status={getConnectionStatus()} isLive={isLive} />
        </View>
      </View>

      {/* Right side - Stream metrics */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[6]]}>
        {isLive && (
          <>
            <MetricItem
              icon={Users}
              label="Viewers"
              value={(viewers || 0).toLocaleString()}
            />
            <MetricItem
              icon={Activity}
              label="Segments"
              value={`${segmentTiming.timeBetweenSegments || 0}ms`}
              status={
                segmentTiming.connectionQuality === "good"
                  ? "good"
                  : segmentTiming.connectionQuality === "degraded"
                    ? "warning"
                    : "error"
              }
            />
            <MetricItem
              icon={Monitor}
              label="Quality"
              value={segmentTiming.connectionQuality.toUpperCase()}
              status={
                segmentTiming.connectionQuality === "good"
                  ? "good"
                  : segmentTiming.connectionQuality === "degraded"
                    ? "warning"
                    : "error"
              }
            />
            <MetricItem
              icon={Radio}
              label="Connection"
              value={ingestConnectionState || "disconnected"}
            />
            <MetricItem
              icon={Wifi}
              label="Range"
              value={segmentTiming.range ? `${segmentTiming.range}ms` : "N/A"}
            />
            <MetricItem icon={Signal} label="Uptime" value={getUptime()} />
          </>
        )}

        {!isLive && (
          <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
            <Radio size={16} color="#6b7280" />
            <Text style={[text.gray[400], { fontSize: 13 }]}>
              Stream offline
            </Text>
          </View>
        )}
      </View>
    </View>
  );
}
