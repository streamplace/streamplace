import { Spinner } from "components/loading/loading";
import { View } from "tamagui";
import { PlayerProps, PlayerStatus } from "./props";

export default function PlayerLoading(props: PlayerProps) {
  if (props.status === PlayerStatus.PLAYING) {
    return <></>;
  }
  return (
    <View
      position="absolute"
      width="100%"
      height="100%"
      zIndex={998}
      alignItems="center"
      justifyContent="center"
      backgroundColor="rgba(0,0,0,0.8)"
    >
      <Spinner></Spinner>
    </View>
  );
}
