import { useNavigation } from "@react-navigation/native";
import { Button, Text, View, zero } from "@streamplace/components";
import { flex } from "@streamplace/components/src/ui";
import { Camera, FerrisWheel } from "@tamagui/lucide-icons";
import { Redirect } from "components/aqlink";
import Loading from "components/loading/loading";
import {
  selectIsReady,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import React, { useState } from "react";
import { useAppSelector } from "store/hooks";
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
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);
  const navigation = useNavigation();

  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }

  if (selectedMode === "webcam") {
    navigation.navigate("MobileGoLive");
  }

  if (selectedMode === "streamkey") {
    return (
      <View flex={1} style={[flex.grow[1], { width: "100%" }]}>
        <View padding="md" direction="row" justify="between" align="end">
          <Button variant="ghost" onPress={() => setSelectedMode(null)}>
            ← Back
          </Button>
          <Text variant="h4" weight="bold">
            Stream from OBS
          </Text>
          <Button variant="ghost" style={{ opacity: 0 }}>
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
