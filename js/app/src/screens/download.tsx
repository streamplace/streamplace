import { Text, zero } from "@streamplace/components";
import { Linking, Pressable, View } from "react-native";
import GetApps from "../../components/get-apps";

const H4 = ({ style, ...props }: any) => (
  <Text style={[{ fontSize: 18, fontWeight: "100" }, style]} {...props} />
);
const P = (props: any) => (
  <Text style={[{ fontSize: 13, marginBottom: 20 }, props.style]} {...props} />
);

const Anchor = ({
  href,
  children,
  style,
  ...props
}: {
  href: string;
  children: React.ReactNode;
  style?: any;
}) => {
  return (
    <Pressable onPress={() => Linking.openURL(href)} {...props}>
      <Text
        style={[
          {
            color: zero.colors.blue[400],
            textDecorationLine: "underline",
            textAlign: "center",
          },
          zero.layout.flex.center,
        ]}
      >
        {children}
      </Text>
    </Pressable>
  );
};

export default function DownloadScreen() {
  return (
    <View
      style={[
        { maxWidth: 500, marginHorizontal: "auto" },
        zero.flex.values[1],
        zero.px[4],
        zero.py[8],
        zero.layout.flex.center,
        zero.gap.all[6],
      ]}
    >
      <GetApps />
      <Text>Or:</Text>
      <Anchor href="https://git.stream.place/streamplace/streamplace/-/releases">
        Get the latest releases for Windows, Mac, and Linux from
        git.stream.place
      </Anchor>
    </View>
  );
}
