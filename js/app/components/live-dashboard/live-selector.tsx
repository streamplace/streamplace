import { useNavigation, useRoute } from "@react-navigation/native";
import { Button, Text, View, zero } from "@streamplace/components";
import { flex } from "@streamplace/components/src/ui";
import Loading from "components/loading/loading";
import { Camera, FerrisWheel } from "lucide-react-native";
import React, { useEffect, useState } from "react";
import { Linking, Pressable } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";
import { StreamKeyScreen } from "./stream-key";

const { layout, gap } = zero;

const elems = [
  {
    title: "Stream your camera",
    Icon: Camera,
    key: "webcam",
  },
  {
    title: "Stream from OBS",
    Icon: FerrisWheel,
    key: "streamkey",
  },
];

export default function StreamScreen({ route }) {
  const [selectedMode, setSelectedMode] = useState<string | null>(null);
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const navigation = useNavigation();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const currentRoute = useRoute();

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

  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return <Loading />;
  }

  if (selectedMode === "webcam") {
    navigation.navigate("MobileGoLive");
  }

  if (selectedMode === "streamkey") {
    return (
      <View flex={1} style={[flex.grow[1], { width: "100%" }]}>
        <View padding="md" direction="row" justify="around" align="start">
          <Button
            variant="ghost"
            width="min"
            onPress={() => setSelectedMode(null)}
          >
            ← Back
          </Button>
          <View align="center">
            <Text variant="h4" weight="bold">
              Stream from OBS
            </Text>
            <Pressable
              onPress={() => Linking.openURL("https://obsproject.com/")}
            >
              <Text
                style={{ color: "#0066cc", textDecorationLine: "underline" }}
              >
                obsproject.com
              </Text>
            </Pressable>
          </View>
          <Button variant="ghost" width="min" style={{ opacity: 0 }}>
            ← Back
          </Button>
        </View>
        <StreamKeyScreen />
      </View>
    );
  }

  return (
    <View
      flex={1}
      justify="around"
      align="stretch"
      padding="md"
      direction="row"
    >
      <View flex={1} align="stretch" justify="center" style={[gap.all[4]]}>
        {elems.map(({ Icon, title, key }, i) => (
          <React.Fragment key={i}>
            <Button
              onPress={() => setSelectedMode(key)}
              variant="primary"
              size="xl"
              style={[{ flexGrow: 0 }, layout.flex.column]}
              leftIcon={<Icon size={24} color="white" />}
            >
              {title}
            </Button>
          </React.Fragment>
        ))}
      </View>
    </View>
  );
}
