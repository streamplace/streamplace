import { zero } from "@streamplace/components";
import {
  AlertTriangle,
  Eye,
  MessageCircle,
  Shield,
} from "@tamagui/lucide-icons";
import { Pressable, Text, View } from "react-native";

const { flex, bg, r, borders, p, text, layout, gap, mb } = zero;

interface ModActionsProps {
  isLive: boolean;
}

export default function ModActions({ isLive }: ModActionsProps) {
  const actions = [
    { icon: Shield, label: "Ban User", color: "red" },
    { icon: MessageCircle, label: "Timeout", color: "yellow" },
    { icon: Eye, label: "Monitor", color: "blue" },
    { icon: AlertTriangle, label: "Report", color: "orange" },
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
      <Text style={[text.white, mb[4], { fontSize: 18, fontWeight: "600" }]}>
        Moderation
      </Text>

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
            disabled={!isLive}
          >
            <action.icon size={20} color={isLive ? "#ffffff" : "#6b7280"} />
            <Text
              style={[
                isLive ? text.white : text.gray[400],
                { fontSize: 14, fontWeight: "500" },
              ]}
            >
              {action.label}
            </Text>
          </Pressable>
        ))}
      </View>

      {!isLive && (
        <Text
          style={[
            text.gray[500],
            { fontSize: 12, textAlign: "center", marginTop: 16 },
          ]}
        >
          Moderation tools available when live
        </Text>
      )}
    </View>
  );
}
