import { Camera, FerrisWheel } from "@tamagui/lucide-icons";
import { Redirect } from "components/aqlink";
import Loading from "components/loading/loading";
import { Player } from "components/mobile/player";
import { FullscreenProvider } from "contexts/FullscreenContext";
import {
  selectIsReady,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import React, { useState } from "react";
import { useAppSelector } from "store/hooks";
import { Button, H6, Text, View } from "tamagui";
import { StreamKeyScreen } from "./stream-key";
const elems = [
  {
    title: "Stream your camera!",
    Icon: Camera,
    key: "webcam",
  },
  {
    title: "Stream from OBS!",
    Icon: FerrisWheel,
    key: "streamkey",
  },
];

export default function StreamScreen({ route }) {
  const [selectedMode, setSelectedMode] = useState<string | null>(null);
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);

  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }

  if (selectedMode === "webcam") {
    return (
      <View f={1}>
        <View
          padding="$3"
          flexDirection="row"
          justifyContent="space-between"
          alignItems="center"
        >
          <Button onPress={() => setSelectedMode(null)}>← Back</Button>
          <Text fontSize="$6" fontWeight="bold">
            Stream your camera
          </Text>
          <View />
        </View>
        <FullscreenProvider>
          <Player ingest src={userProfile.did} name={userProfile.handle} />
        </FullscreenProvider>
      </View>
    );
  }

  if (selectedMode === "streamkey") {
    return (
      <View f={1}>
        <View
          padding="$3"
          flexDirection="row"
          justifyContent="space-between"
          alignItems="center"
        >
          <Button onPress={() => setSelectedMode(null)}>← Back</Button>
          <Text fontSize="$6" fontWeight="bold">
            Stream from OBS
          </Text>
          <View />
        </View>
        <StreamKeyScreen />
      </View>
    );
  }

  return (
    <View f={1} jc="space-around" ai="stretch" padding="$3" flexDirection="row">
      <View f={1} maxWidth={250} alignItems="stretch" justifyContent="center">
        {elems.map(({ Icon, title, key }, i) => (
          <React.Fragment key={i}>
            <Button
              onPress={() => setSelectedMode(key)}
              style={{ display: "flex", flex: 1, flexGrow: 0, flexBasis: 75 }}
            >
              <View
                f={1}
                flexDirection="row"
                ai="center"
                jc="space-between"
                backgroundColor="$accentColor"
                borderRadius="$10"
              >
                <View padding="$5" paddingRight={0}>
                  <Icon size={48} />
                </View>
                <Text f={1} textAlign="right" paddingRight="$5">
                  {title}
                </Text>
              </View>
            </Button>
            {i < elems.length - 1 && (
              <View jc="center" ai="center">
                <H6 padding="$5">OR</H6>
              </View>
            )}
          </React.Fragment>
        ))}
      </View>
    </View>
  );
}
