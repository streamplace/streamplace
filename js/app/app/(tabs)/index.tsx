import { ExternalLink } from "@tamagui/lucide-icons";
import {
  Anchor,
  H1,
  H2,
  H3,
  Image,
  Paragraph,
  XStack,
  YStack,
  styled,
  View,
  Button,
  ScrollView,
  useWindowDimensions,
  isWeb,
  Text,
} from "tamagui";
const CodeH3 = styled(H3, { fontFamily: "$mono" });
const CenteredH1 = styled(H1, {
  fontWeight: "$2",
  textAlign: "center",
  fontSize: isWeb ? "$16" : "$5",
  // flex: 1,
  lineHeight: "$16",
} as const);
const CenteredH2 = styled(H2, {
  fontWeight: "$2",
  textAlign: "center",
  // lineHeight: "$6",
  fontSize: "$10",
});
const CenteredH3 = styled(H3, {
  fontWeight: "$2",
  textAlign: "center",
  fontSize: "$8",
});
const CubeImage = styled(Image, {
  width: 100,
  height: 100,
  resizeMethod: "scale",
});
import { WebView } from "react-native-webview";
import { Countdown } from "components";
import { ImageBackground } from "react-native";
import * as env from "constants/env";
import { useState } from "react";
import GetApps from "components/get-apps";
import { Link } from "expo-router";
import StreamList from "components/stream-list/stream-list";

const WebviewIframe = ({ src }) => {
  if (isWeb) {
    return (
      <iframe
        allowFullScreen={true}
        src={src}
        style={{ border: 0, flex: 1 }}
      ></iframe>
    );
  } else {
    return (
      <WebView
        allowsInlineMediaPlayback={true}
        scrollEnabled={false}
        source={{ uri: src }}
        style={{ flex: 1, backgroundColor: "transparent" }}
      />
    );
  }
};

const TAP_COUNT = 5;
const TAP_WINDOW = 5000;
export default function TabOneScreen() {
  // const isLive = Date.now() >= 1721149200000;
  const [debug, setDebug] = useState(false);
  const [presses, setPresses] = useState<number[]>([]);
  const handlePress = () => {
    const newTaps = [...presses, Date.now()];
    if (newTaps.length > TAP_COUNT) {
      newTaps.shift();
    }
    if (
      newTaps.length >= TAP_COUNT &&
      newTaps[newTaps.length - 1] - newTaps[0] <= TAP_WINDOW
    ) {
      setPresses([]);
      setDebug(!debug);
    } else {
      setPresses(newTaps);
    }
  };
  return (
    <YStack f={1} ai="center" gap="$8" pt="$5" alignItems="stretch">
      {/* <YStack f={1} alignItems="stretch">
        <View fg={1} flexBasis={0} onPress={handlePress}>
          {!debug && (
            <ImageBackground
              source={require("assets/images/cube.png")}
              style={{ width: "100%", height: "100%" }}
              resizeMode="contain"
            ></ImageBackground>
          )}
          {debug &&
            Object.entries(env).map(([k, v]) => (
              <Text key={k}>
                {k}={v}
              </Text>
            ))}
        </View>
      </YStack> */}
      {/* <View flexShrink={0} flexGrow={0}>
        <CenteredH2>Aquareum: The Video Layer for Everything</CenteredH2>
      </View>
      <View>
        <GetApps />
      </View> */}
      <StreamList></StreamList>
    </YStack>
  );
}
