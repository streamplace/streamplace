import { Button, Text, useTheme, View } from "@streamplace/components";
import { LogoMark } from "components/brand/logo";
import {
  MessagesSquare,
  MonitorPlay,
  Network,
  ShieldCheck,
  Terminal,
  Zap,
} from "lucide-react-native";
import { Linking, ScrollView } from "react-native";

const FEATURES = [
  {
    icon: Terminal,
    title: "Open-source, single-binary node",
    body: "Get up and running with one command. No complex configuration or deep video expertise required — built for hackers and builders.",
  },
  {
    icon: ShieldCheck,
    title: "User sovereignty by design",
    body: "Every video is cryptographically signed by its creator and respects their consent preferences — on the same public-key infrastructure as the social web.",
  },
  {
    icon: MonitorPlay,
    title: "A familiar streaming experience",
    body: "Native apps for iOS, Android, and web with the rich video features people expect: livestreaming, clips, uploads, and more.",
  },
  {
    icon: Network,
    title: "Built for federation",
    body: "Integrates seamlessly with the AT Protocol. Streamplace nodes connect to any compatible network to index and serve video content.",
  },
  {
    icon: Zap,
    title: "Powered by Livepeer",
    body: "Leverages battle-tested decentralized video infrastructure for transcoding, distribution, and delivery at scale.",
  },
];

function FeatureRow({
  icon: Icon,
  title,
  body,
}: {
  icon: typeof Terminal;
  title: string;
  body: string;
}) {
  const { theme } = useTheme();
  const c = theme.colors;
  return (
    <View style={{ flexDirection: "row", gap: 16, alignItems: "flex-start" }}>
      <View
        style={{
          width: 44,
          height: 44,
          borderRadius: 12,
          alignItems: "center",
          justifyContent: "center",
          backgroundColor: c.surface1,
          borderWidth: 1,
          borderColor: c.borderSubtle,
        }}
      >
        <Icon size={20} color={c.text2} />
      </View>
      <View style={{ flex: 1, gap: 4, paddingTop: 2 }}>
        <Text weight="semibold" style={{ color: c.text1, fontSize: 16 }}>
          {title}
        </Text>
        <Text style={{ color: c.text3, fontSize: 14.5, lineHeight: 22 }}>
          {body}
        </Text>
      </View>
    </View>
  );
}

export default function AboutScreen() {
  const { theme } = useTheme();
  const c = theme.colors;
  return (
    <ScrollView
      contentContainerStyle={{
        alignItems: "center",
        paddingHorizontal: 24,
        paddingVertical: 56,
      }}
    >
      <View style={{ width: "100%", maxWidth: 680, gap: 44 }}>
        {/* Hero */}
        <View style={{ gap: 18 }}>
          <LogoMark size={44} color={c.text1} />
          <View style={{ gap: 14 }}>
            <Text
              weight="semibold"
              style={{
                fontSize: 12,
                letterSpacing: 1.2,
                textTransform: "uppercase",
                color: c.primary,
              }}
            >
              What is Streamplace?
            </Text>
            <Text
              weight="semibold"
              style={{
                color: c.text1,
                fontSize: 34,
                lineHeight: 40,
                letterSpacing: -0.5,
                maxWidth: 560,
              }}
            >
              The video layer for the open social web.
            </Text>
            <Text
              style={{
                color: c.text2,
                fontSize: 17,
                lineHeight: 26,
                maxWidth: 600,
              }}
            >
              Open-source infrastructure bringing high-quality, creator-owned
              video to the AT Protocol — designed around user sovereignty and
              content authenticity.
            </Text>
          </View>
        </View>

        {/* Feature list */}
        <View style={{ gap: 28 }}>
          {FEATURES.map((f) => (
            <FeatureRow key={f.title} {...f} />
          ))}
        </View>

        {/* Get-involved CTA */}
        <View
          style={{
            padding: 28,
            borderRadius: 16,
            backgroundColor: c.surface1,
            borderWidth: 1,
            borderColor: c.borderSubtle,
            alignItems: "center",
            gap: 8,
          }}
        >
          <Text weight="semibold" style={{ color: c.text1, fontSize: 18 }}>
            Want to get involved?
          </Text>
          <Text
            style={{
              color: c.text3,
              fontSize: 14.5,
              lineHeight: 22,
              textAlign: "center",
              maxWidth: 440,
            }}
          >
            Join our Discord to learn more about Streamplace and how you can
            help build it.
          </Text>
          <View style={{ marginTop: 14 }}>
            <Button
              width="min"
              leftIcon={<MessagesSquare size={16} />}
              onPress={() => Linking.openURL("https://discord.stream.place")}
            >
              Join our Discord
            </Button>
          </View>
        </View>
      </View>
    </ScrollView>
  );
}
