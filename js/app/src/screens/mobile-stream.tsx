import { useNavigation } from "@react-navigation/native";
import {
  KeepAwake,
  LivestreamProvider,
  PlayerProvider,
  Text,
  useLivestreamStore,
} from "@streamplace/components";
import { colors, surfaces } from "@streamplace/components/src/lib/theme/tokens";
import { Player } from "components/mobile/player";
import { PlayerProps } from "components/player/props";
import { FullscreenProvider } from "contexts/FullscreenContext";
import useTitle from "hooks/useTitle";
import { Platform, View } from "react-native";
import { queryToProps } from "./util";

const isWeb = Platform.OS === "web";

function StreamError({ message }: { message: string }) {
  return (
    <View
      style={{
        flex: 1,
        justifyContent: "center",
        alignItems: "center",
        backgroundColor: surfaces.dark[1],
      }}
    >
      <Text style={{ color: colors.white, fontSize: 18 }}>{message}</Text>
    </View>
  );
}

function MobileStreamInner({
  user,
  src,
  extraProps,
  onTeleport,
}: {
  user: string;
  src: string;
  extraProps: Partial<PlayerProps>;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
}) {
  const problems = useLivestreamStore((x) => x.problems);

  const userNotFoundError = problems.find((p) => p.code === "user_not_found");

  useTitle(user);

  if (userNotFoundError) {
    return <StreamError message={userNotFoundError.message} />;
  }

  return (
    <>
      <KeepAwake />
      <FullscreenProvider>
        <Player key={src} src={src} {...extraProps} onTeleport={onTeleport} />
      </FullscreenProvider>
    </>
  );
}

export default function MobileStream({ route }) {
  const { user, protocol, url } = route?.params ?? {};
  let navi = useNavigation();
  let extraProps: Partial<PlayerProps> = {};
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  let src = user;
  if (user === "stream") {
    src = url;
  }

  const handleTeleport = (targetHandle: string, targetDID?: string) => {
    if (!navi || (!targetHandle && !targetDID)) {
      console.error("Navigation or target info missing for teleport");
      return;
    }
    navi.navigate("Stream", {
      user: targetHandle,
    });
  };

  return (
    <LivestreamProvider key={src} src={src} onTeleport={handleTeleport}>
      <PlayerProvider>
        <MobileStreamInner
          user={user}
          src={src}
          extraProps={extraProps}
          onTeleport={handleTeleport}
        />
      </PlayerProvider>
    </LivestreamProvider>
  );
}
