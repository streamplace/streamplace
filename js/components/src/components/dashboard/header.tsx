import { Car, Radio, Users } from "lucide-react-native";
import { Text, View } from "react-native";
import * as zero from "../../ui";

const { bg, r, borders, px, py, text, layout, gap } = zero;

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
  isLive: boolean;
  streamTitle?: string;
  viewers?: number;
  uptime?: string;
  bitrate?: string;
  timeBetweenSegments?: number;
  connectionStatus?: "excellent" | "good" | "poor" | "offline";
}

export default function Header({
  isLive,
  streamTitle = "Live Stream",
  viewers = 0,
  uptime = "00:00:00",
  bitrate = "0 mbps",
  timeBetweenSegments = 0,
  connectionStatus = "offline",
}: HeaderProps) {
  const getConnectionQuality = (): "good" | "warning" | "error" => {
    if (timeBetweenSegments <= 1500) return "good";
    if (timeBetweenSegments <= 3000) return "warning";
    return "error";
  };

  return (
    <View
      style={[
        px[4],
        py[3],
        r.lg,
        layout.flex.row,
        layout.flex.spaceBetween,
        bg.neutral[900],
        borders.width.thin,
        borders.color.neutral[700],
      ]}
    >
      {/* Left side - Stream title and status */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[4]]}>
        <View>
          <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
            {streamTitle}
          </Text>
          <StatusIndicator status={connectionStatus} isLive={isLive} />
        </View>
      </View>

      {/* Right side - Stream metrics */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[6]]}>
        {isLive && (
          <>
            <MetricItem
              icon={Users}
              label="Viewers"
              value={viewers.toLocaleString()}
            />
            <MetricItem icon={Car} label="Bitrate" value={bitrate} />
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
