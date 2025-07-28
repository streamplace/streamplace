import {
  KeepAwake,
  PlayerStatus,
  usePlayerStore,
} from "@streamplace/components";
import { Play } from "@tamagui/lucide-icons";
import { Spinner } from "components/loading/loading";
import { useTheme, View } from "tamagui";

export default function PlayerLoading() {
  const status = usePlayerStore((x) => x.status);
  const theme = useTheme();

  if (status === PlayerStatus.PLAYING) {
    return <KeepAwake />;
  }

  let spinner = <Spinner></Spinner>;
  if (status === PlayerStatus.PAUSE) {
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
