import { Button, Text, useTheme, View } from "@streamplace/components";
import { LogoMark } from "components/brand/logo";
import { MonitorDown } from "lucide-react-native";
import { Linking, ScrollView } from "react-native";
import GetApps from "../../components/get-apps";

const RELEASES_URL =
  "https://git.stream.place/streamplace/streamplace/-/releases";

function SectionLabel({ children }: { children: string }) {
  const { theme } = useTheme();
  return (
    <Text
      weight="semibold"
      style={{
        fontSize: 11,
        letterSpacing: 1.4,
        textTransform: "uppercase",
        color: theme.colors.text3,
      }}
    >
      {children}
    </Text>
  );
}

export default function DownloadScreen() {
  const { theme } = useTheme();
  const c = theme.colors;

  const card = {
    width: "100%" as const,
    padding: 28,
    borderRadius: 16,
    backgroundColor: c.surface1,
    borderWidth: 1,
    borderColor: c.borderSubtle,
    alignItems: "center" as const,
    gap: 18,
  };

  return (
    <ScrollView
      contentContainerStyle={{
        alignItems: "center",
        paddingHorizontal: 24,
        paddingVertical: 56,
      }}
    >
      <View style={{ width: "100%", maxWidth: 540, gap: 32, alignItems: "center" }}>
        {/* Hero */}
        <View style={{ alignItems: "center", gap: 16 }}>
          <LogoMark size={48} color={c.text1} />
          <View style={{ alignItems: "center", gap: 10 }}>
            <Text
              weight="semibold"
              style={{
                color: c.text1,
                fontSize: 30,
                letterSpacing: -0.4,
                textAlign: "center",
              }}
            >
              Get Streamplace
            </Text>
            <Text
              style={{
                color: c.text2,
                fontSize: 16,
                lineHeight: 24,
                textAlign: "center",
                maxWidth: 420,
              }}
            >
              Watch, stream, and manage your channel from anywhere — on mobile,
              desktop, or the web.
            </Text>
          </View>
        </View>

        {/* Mobile */}
        <View style={card}>
          <SectionLabel>Mobile</SectionLabel>
          <GetApps />
        </View>

        {/* Desktop */}
        <View style={card}>
          <SectionLabel>Desktop</SectionLabel>
          <View
            style={{
              width: 56,
              height: 56,
              borderRadius: 14,
              alignItems: "center",
              justifyContent: "center",
              backgroundColor: c.surface2,
              borderWidth: 1,
              borderColor: c.borderSubtle,
            }}
          >
            <MonitorDown size={26} color={c.text2} />
          </View>
          <Text
            style={{
              color: c.text3,
              fontSize: 14.5,
              textAlign: "center",
            }}
          >
            Windows · macOS · Linux
          </Text>
          <Button
            width="min"
            leftIcon={<MonitorDown size={16} />}
            onPress={() => Linking.openURL(RELEASES_URL)}
            // width="min" → alignSelf:flex-start; center it within this column.
            style={{ alignSelf: "center" }}
          >
            Get the latest release
          </Button>
        </View>
      </View>
    </ScrollView>
  );
}
