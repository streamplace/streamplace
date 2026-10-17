import { Button } from "@streamplace/components";
import { View } from "react-native";

export default function AppReturnScreen({ route }) {
  const scheme = route.params?.scheme;
  // This should work. It does work on iOS. On Android, it causes the token to be
  // authorized twice? I don't know why. I spent two hours trying to figure out why
  // without luck. You're welcome to try more if you want! But it only matters for the
  // login flow on the localhost development case so who cares I guess.
  // useEffect(() => {
  //   document.location.href = `${scheme}:/app-return${document.location.search}`;
  // }, []);
  return (
    <View
      style={[
        { flex: 1 },
        { alignItems: "center" },
        { justifyContent: "center" },
      ]}
    >
      <Button
        size="lg"
        width="min"
        onPress={() => {
          document.location.href = `${scheme}:/${document.location.search}`;
        }}
      >
        Complete login
      </Button>
    </View>
  );
}
