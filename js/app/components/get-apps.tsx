import { Anchor, Image, XStack } from "tamagui";

const RATIO = 3.39741547176;
const WIDTH = 200;
const HEIGHT = 200 / RATIO;

export default function GetApps() {
  return (
    <XStack justifyContent="center">
      <Anchor
        target="_blank"
        href="https://apps.apple.com/us/app/streamplace/id6535653195"
      >
        <Image
          width={WIDTH}
          height={HEIGHT}
          mx="$2"
          source={require("../assets/images/appstore.svg")}
        />
      </Anchor>
      <Anchor
        target="_blank"
        href="https://play.google.com/store/apps/details?id=tv.aquareum"
      >
        <Image
          width={WIDTH}
          height={HEIGHT}
          mx="$2"
          source={require("../assets/images/playstore.svg")}
        />
      </Anchor>
    </XStack>
  );
}
