import { H4 as TamaguiH4, Paragraph, View, Anchor } from "tamagui";
import GetApps from "../../components/get-apps";

const H4 = (props: any) => <TamaguiH4 fontWeight="100" {...props} />;
const P = (props: any) => (
  <Paragraph fontSize={13} marginBottom={20} {...props} />
);

export default function AboutScreen() {
  return (
    <View maxWidth={500} marginHorizontal="auto" f={1} ai="center" jc="center">
      <GetApps />
      <H4 padding="$10" textAlign="center">
        Or:
      </H4>
      <Anchor
        fontSize={20}
        textAlign="center"
        href="https://git.aquareum.tv/streamplace/streamplace/-/releases"
      >
        Get the latest releases for Windows, Mac, and Linux from git.aquareum.tv
      </Anchor>
    </View>
  );
}
