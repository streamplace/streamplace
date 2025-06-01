import Loading from "components/loading/loading";
import { Text, View } from "tamagui";

export default function Waiting() {
  return (
    <View marginTop="$10" flexDirection="row" ai="center" jc="center">
      <Loading />
      <Text marginLeft="$5">Waiting for stream to start...</Text>
    </View>
  );
}
