import { useNavigation } from "@react-navigation/native";
import {
  KeepAwake,
  LivestreamProvider,
  PlayerProvider,
} from "@streamplace/components";
import { Player } from "components/mobile/player";
import { PlayerProps } from "components/player/props";
import { FullscreenProvider } from "contexts/FullscreenContext";
import useTitle from "hooks/useTitle";
import { Platform } from "react-native";
import { queryToProps } from "./util";

const isWeb = Platform.OS === "web";

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
  useTitle(user);

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
