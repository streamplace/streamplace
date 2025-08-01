import { zero } from "@streamplace/components";
import { Heart, MessageCircle, Users } from "@tamagui/lucide-icons";
import { Text, View } from "react-native";

const { flex, bg, r, p, text, layout, gap, borderRadius } = zero;

// Mock data - replace with real data from your stores
const mockStats = {
  viewers: 1337,
  likes: 89,
  messages: 234,
};

interface StatCardProps {
  icon: any;
  label: string;
  value: string;
  color: "blue" | "red" | "green";
}

function StatCard({ icon: Icon, label, value, color }: StatCardProps) {
  const colors = {
    blue: { bg: bg.blue[500], text: text.blue[100] },
    red: { bg: bg.red[500], text: text.red[100] },
    green: { bg: bg.green[500], text: text.green[100] },
  };

  return (
    <View
      style={[
        flex.values[1],
        colors[color].bg,
        r[3],
        p[4],
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[3],
        borderRadius["2xl"],
      ]}
    >
      <Icon size={24} color="white" />
      <View>
        <Text style={[text.white, { fontSize: 24, fontWeight: "700" }]}>
          {value}
        </Text>
        <Text style={[colors[color].text, { fontSize: 12, fontWeight: "500" }]}>
          {label}
        </Text>
      </View>
    </View>
  );
}

export default function QuickStats() {
  return (
    <View
      style={[flex.values[1], gap.all[4], layout.flex.row, { maxHeight: 64 }]}
    >
      <StatCard
        icon={Users}
        label="Viewers"
        value={mockStats.viewers.toLocaleString()}
        color="blue"
      />
      <StatCard
        icon={Heart}
        label="Likes"
        value={mockStats.likes.toString()}
        color="red"
      />
      <StatCard
        icon={MessageCircle}
        label="Messages"
        value={mockStats.messages.toString()}
        color="green"
      />
    </View>
  );
}
