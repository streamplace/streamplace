import {
  KeepAwake,
  LivestreamProvider,
  PlayerProvider,
  useLivestreamStore,
} from "@streamplace/components";
import { Player } from "components/mobile/player";
import { PlayerProps } from "components/player/props";
import { FullscreenProvider } from "contexts/FullscreenContext";
import useTitle from "hooks/useTitle";
import { Platform, Text, View } from "react-native";
import { queryToProps } from "./util";

const isWeb = Platform.OS === "web";

function StreamError({ message }: { message: string }) {
  return (
    <View
      style={{
        flex: 1,
        justifyContent: "center",
        alignItems: "center",
        backgroundColor: "#111",
      }}
    >
      <Text style={{ color: "#fff", fontSize: 18 }}>{message}</Text>
    </View>
  );
}

function MobileStreamInner({
  user,
  src,
  extraProps,
}: {
  user: string;
  src: string;
  extraProps: Partial<PlayerProps>;
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
        <Player src={src} {...extraProps} />
      </FullscreenProvider>
    </>
  );
}

export default function MobileStream({ route }) {
  const { user, protocol, url } = route.params;
  let extraProps: Partial<PlayerProps> = {};
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  let src = user;
  if (user === "stream") {
    src = url;
  }

  return (
    <LivestreamProvider src={src}>
      <PlayerProvider>
        <MobileStreamInner user={user} src={src} extraProps={extraProps} />
      </PlayerProvider>
    </LivestreamProvider>
  );
}
