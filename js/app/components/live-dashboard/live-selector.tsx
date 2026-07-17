import { useNavigation, useRoute } from "@react-navigation/native";
import { hexToRgba, Text, useTheme } from "@streamplace/components";
import { spacing } from "@streamplace/components/src/lib/theme/tokens";
import Loading from "components/loading/loading";
import { ArrowLeft, Camera, MonitorPlay, Radio } from "lucide-react-native";
import React, { useEffect, useState } from "react";
import { Pressable, View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";
import { StreamKeyScreen } from "./stream-key";

// A single "how do you want to send video" choice, rendered as a calm card
// rather than a loud primary button — hover lifts the surface + border.
function SourceCard({
  Icon,
  title,
  description,
  onPress,
}: {
  Icon: any;
  title: string;
  description: string;
  onPress: () => void;
}) {
  const { theme } = useTheme();
  const [hover, setHover] = useState(false);
  return (
    <Pressable
      onPress={onPress}
      onHoverIn={() => setHover(true)}
      onHoverOut={() => setHover(false)}
      style={{
        flex: 1,
        alignItems: "center",
        gap: 12,
        paddingVertical: spacing[5],
        paddingHorizontal: spacing[4],
        borderRadius: theme.borderRadius.lg,
        borderCurve: "continuous" as const,
        borderWidth: 1,
        borderColor: hover
          ? theme.colors.borderStrong
          : theme.colors.borderSubtle,
        backgroundColor: hover ? theme.colors.surface2 : theme.colors.surface1,
      }}
    >
      <View
        style={{
          width: 40,
          height: 40,
          borderRadius: 11,
          borderCurve: "continuous" as const,
          alignItems: "center",
          justifyContent: "center",
          backgroundColor: hexToRgba(theme.colors.primary, 0.14),
        }}
      >
        <Icon size={20} color={theme.colors.primary} />
      </View>
      <View style={{ alignItems: "center", gap: 3 }}>
        <Text size="sm" weight="medium" style={{ color: theme.colors.text1 }}>
          {title}
        </Text>
        <Text
          size="xs"
          style={{ color: theme.colors.text3, textAlign: "center" }}
        >
          {description}
        </Text>
      </View>
    </Pressable>
  );
}

export default function StreamScreen({ route }: { route?: any }) {
  const [selectedMode, setSelectedMode] = useState<string | null>(null);
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const navigation = useNavigation();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const currentRoute = useRoute();
  const { theme } = useTheme();

  useEffect(() => {
    if (isReady && !userProfile) {
      openLoginModal({ name: currentRoute.name, params: currentRoute.params });
    }
  }, [
    isReady,
    userProfile,
    openLoginModal,
    currentRoute.name,
    currentRoute.params,
  ]);

  if (!isReady || !userProfile) {
    return <Loading />;
  }

  if (selectedMode === "webcam") {
    navigation.navigate("MobileGoLive" as never);
  }

  if (selectedMode === "streamkey") {
    return (
      <View style={{ flex: 1, width: "100%" }}>
        <View
          style={{
            flexDirection: "row",
            alignItems: "center",
            gap: 10,
            paddingHorizontal: spacing[4],
            paddingVertical: spacing[3],
          }}
        >
          <Pressable
            onPress={() => setSelectedMode(null)}
            style={{
              flexDirection: "row",
              alignItems: "center",
              gap: 4,
              paddingVertical: 4,
              paddingHorizontal: 6,
              marginLeft: -6,
              borderRadius: theme.borderRadius.md,
            }}
          >
            <ArrowLeft size={16} color={theme.colors.text2} />
            <Text
              size="sm"
              weight="medium"
              style={{ color: theme.colors.text2 }}
            >
              Back
            </Text>
          </Pressable>
          <Text
            size="sm"
            weight="semibold"
            style={{ color: theme.colors.text1 }}
          >
            Stream from OBS
          </Text>
        </View>
        <StreamKeyScreen />
      </View>
    );
  }

  return (
    <View
      style={{
        flex: 1,
        alignItems: "center",
        justifyContent: "center",
        padding: spacing[6],
        gap: spacing[6],
      }}
    >
      <View style={{ alignItems: "center", gap: spacing[4] }}>
        <View
          style={{
            width: 52,
            height: 52,
            borderRadius: 999,
            alignItems: "center",
            justifyContent: "center",
            backgroundColor: theme.colors.surface2,
            borderWidth: 1,
            borderColor: theme.colors.borderSubtle,
          }}
        >
          <Radio size={22} color={theme.colors.text3} />
        </View>
        <View style={{ alignItems: "center", gap: 4 }}>
          <Text
            size="lg"
            weight="semibold"
            style={{ color: theme.colors.text1 }}
          >
            You're offline
          </Text>
          <Text size="sm" style={{ color: theme.colors.text3 }}>
            Connect a source to go live
          </Text>
        </View>
      </View>
      <View
        style={{
          flexDirection: "row",
          gap: spacing[3],
          width: "100%",
          maxWidth: 460,
        }}
      >
        <SourceCard
          Icon={Camera}
          title="Camera"
          description="Go live from your webcam"
          onPress={() => setSelectedMode("webcam")}
        />
        <SourceCard
          Icon={MonitorPlay}
          title="OBS / Software"
          description="Use a stream key"
          onPress={() => setSelectedMode("streamkey")}
        />
      </View>
    </View>
  );
}
