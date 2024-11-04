import { Play } from "@tamagui/lucide-icons";
import { Spinner } from "components/loading/loading";
import { useTheme, View } from "tamagui";
import { PlayerProps, PlayerStatus } from "./props";

export default function PlayerLoading(props: PlayerProps) {
  if (props.status === PlayerStatus.PLAYING) {
    return <></>;
  }
  let spinner = <Spinner></Spinner>;
  if (props.status === PlayerStatus.PAUSE) {
    const theme = useTheme();
    spinner = <Play size="$12" color={theme.accentColor.val} />;
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
      {spinner}
    </View>
  );
}
