import { Camera, FerrisWheel } from "@tamagui/lucide-icons";
import AQLink from "components/aqlink";
import React from "react";
import { H6, Text, View } from "tamagui";
const elems = [
  {
    title: "Stream your camera!",
    Icon: Camera,
    to: "Webcam",
  },
  {
    title: "Stream from OBS!",
    Icon: FerrisWheel,
    to: "StreamKey",
  },
];

export default function StreamScreen({ route }) {
  return (
    <View f={1} jc="space-around" ai="stretch" padding="$3" flexDirection="row">
      <View f={1} maxWidth={250} alignItems="stretch" justifyContent="center">
        {elems.map(({ Icon, title, to }, i) => (
          <React.Fragment key={i}>
            <AQLink
              to={{ screen: to }}
              style={{ display: "flex", flex: 1, flexGrow: 0, flexBasis: 75 }}
            >
              <View
                f={1}
                flexDirection="row"
                ai="center"
                jc="space-between"
                backgroundColor="$accentColor"
                // padding="$5"
                borderRadius="$10"
              >
                <View padding="$5" paddingRight={0}>
                  <Icon size={48} />
                </View>
                <Text f={1} textAlign="right" paddingRight="$5">
                  {title}
                </Text>
              </View>
            </AQLink>
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
