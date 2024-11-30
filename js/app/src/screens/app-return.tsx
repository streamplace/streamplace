import { useEffect } from "react";
import { Linking } from "react-native";
import { Button, View } from "tamagui";

export default function AppReturnScreen({ route }) {
  const scheme = route.params?.scheme;
  useEffect(() => {
    document.location.href = `${scheme}:/app-return${document.location.search}`;
  }, []);
  return (
    <View f={1} ai="center" jc="center">
      <Button
        backgroundColor="$accentColor"
        fontSize="$8"
        padding="$6"
        onPress={() => {
          document.location.href = `${scheme}:/app-return${document.location.search}`;
        }}
      >
        Complete login
      </Button>
    </View>
  );
}
