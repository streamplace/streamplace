import { Camera, FerrisWheel } from "@tamagui/lucide-icons";
import AQLink from "components/aqlink";
import React from "react";
import { Button, H6, Text, View } from "tamagui";
const elems = [
  {
    title: "Stream your camera!",
    Icon: Camera,
    to: "Webcam",
  },
  {
    title: "Stream from OBS!",
    Icon: FerrisWheel,
    to: "Webcam",
  },
];

export default function StreamScreen({ route }) {
  return (
    <View f={1} jc="space-around" ai="center" padding="$3" flexDirection="row">
      <View f={1} maxWidth={250}>
        {elems.map(({ Icon, title, to }, i) => (
          <React.Fragment key={i}>
            <AQLink to={{ screen: to }} style={{ display: "flex" }}>
              <Button f={1} padding="$6" backgroundColor="$accentColor">
                <View f={1} flexDirection="row" ai="center" jc="space-between">
                  <Icon padding="$5" size={48} marginLeft={-20} />
                  <Text>{title}</Text>
                </View>
              </Button>
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
