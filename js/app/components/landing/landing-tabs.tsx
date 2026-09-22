import { Text, useStreamerLockedOut, useTheme } from "@streamplace/components";
import AQLink from "components/aqlink";
import { View } from "react-native";

// The social shell's landing tabs: the three front pages as a 48px tab bar
// at the top of the feed column (the client design's "Live / Play on demand
// / Go live"). Three equal tabs; the active one is bright with a 2px accent
// underline the width of its label, the others muted. Each is a real link,
// so the pages keep their own paths (/, /video, /live) and cards. Go live
// goes straight to the live dashboard (/live); a viewer the node does not
// let stream gets the go-live page that says who can (/go-live) instead.
export type LandingTab = "live" | "vod" | "golive";

export function LandingTabs({ active }: { active: LandingTab }) {
  const { theme } = useTheme();
  const lockedOut = useStreamerLockedOut();
  const TABS: { key: LandingTab; label: string; to: any }[] = [
    { key: "live", label: "Live", to: { screen: "HomeMain" } },
    { key: "vod", label: "Play on demand", to: { screen: "VideoList" } },
    {
      key: "golive",
      label: "Go live",
      to: { screen: lockedOut ? "GoLiveTab" : "LiveDashboard" },
    },
  ];
  return (
    <View
      style={{
        flexDirection: "row",
        height: 48,
        borderBottomWidth: 1,
        borderBottomColor: theme.colors.borderSubtle,
      }}
    >
      {TABS.map((tab) => {
        const isActive = tab.key === active;
        return (
          <AQLink
            key={tab.key}
            to={tab.to}
            style={{
              flex: 1,
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <View style={{ alignItems: "center", gap: 6, paddingTop: 2 }}>
              <Text
                weight={isActive ? "semibold" : "medium"}
                style={{
                  fontSize: 15,
                  lineHeight: 20,
                  color: isActive ? theme.colors.text1 : theme.colors.text3,
                }}
              >
                {tab.label}
              </Text>
              <View
                style={{
                  height: 2,
                  alignSelf: "stretch",
                  borderRadius: 1,
                  backgroundColor: isActive
                    ? theme.colors.accent
                    : "transparent",
                }}
              />
            </View>
          </AQLink>
        );
      })}
    </View>
  );
}
