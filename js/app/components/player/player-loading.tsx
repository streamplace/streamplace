import { Play } from "@tamagui/lucide-icons";
import KeepAwake from "components/keep-awake";
import { Spinner } from "components/loading/loading";
import { useTheme, View } from "tamagui";
import { PlayerProps, PlayerStatus } from "./props";

export default function PlayerLoading(props: PlayerProps) {
  const theme = useTheme();
  if (props.status === PlayerStatus.PLAYING) {
    return <KeepAwake />;
  }
  let spinner = <Spinner></Spinner>;
  if (props.status === PlayerStatus.PAUSE) {
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
