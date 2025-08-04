import {
  useLivestreamStore,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import {
  AlertTriangle,
  Eye,
  MessageCircle,
  Shield,
} from "@tamagui/lucide-icons";
import { Pressable, Text, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";

const { flex, bg, r, borders, p, text, layout, gap, mb } = zero;

interface ModActionsProps {
  isLive?: boolean;
}

export default function ModActions({ isLive: propIsLive }: ModActionsProps) {
  // Get real data from hooks
  const isUserLive = useLiveUser();
  const chat = useLivestreamStore((x) => x.chat);
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);

  // Use hook data primarily, fallback to props
  const isLive = propIsLive ?? isUserLive;
  const isConnected = ingestConnectionState === "connected";
  const canModerate = isLive && isConnected;
  const actions = [
    {
      icon: Shield,
      label: "Ban User",
      color: "red",
      action: () => console.log("Ban user action"),
    },
    {
      icon: MessageCircle,
      label: "Timeout",
      color: "yellow",
      action: () => console.log("Timeout user action"),
    },
    {
      icon: Eye,
      label: "Monitor",
      color: "blue",
      action: () => console.log("Monitor stream action"),
    },
    {
      icon: AlertTriangle,
      label: "Report",
      color: "orange",
      action: () => console.log("Report content action"),
    },
  ];

  return (
    <View
      style={[
        flex.values[1],
        bg.gray[800],
        r[3],
        borders.width.thin,
        borders.color.gray[700],
        p[4],
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          mb[4],
        ]}
      >
        <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
          Moderation
        </Text>
        {chat && (
          <Text style={[text.gray[400], { fontSize: 12 }]}>
            {chat.length} messages
          </Text>
        )}
      </View>

      <View style={[layout.flex.row, gap.all[3]]}>
        {actions.map((action, index) => (
          <Pressable
            key={index}
            style={[
              flex.grow[1],
              bg.gray[700],
              r[2],
              p[3],
              layout.flex.row,
              layout.flex.alignCenter,
              gap.all[2],
              borders.width.thin,
              borders.color.gray[600],
            ]}
            disabled={!canModerate}
            onPress={action.action}
          >
            <action.icon
              size={20}
              color={canModerate ? "#ffffff" : "#6b7280"}
            />
            <Text
              style={[
                canModerate ? text.white : text.gray[400],
                { fontSize: 14, fontWeight: "500" },
              ]}
            >
              {action.label}
            </Text>
          </Pressable>
        ))}
      </View>

      {!canModerate && (
        <Text
          style={[
            text.gray[500],
            { fontSize: 12, textAlign: "center", marginTop: 16 },
          ]}
        >
          {!isLive
            ? "Moderation tools available when live"
            : !isConnected
              ? "Waiting for stream connection..."
              : "Moderation tools unavailable"}
        </Text>
      )}
    </View>
  );
}
