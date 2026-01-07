import {
  LivestreamProvider,
  Player,
  PlayerProps,
  PlayerProvider,
} from "@streamplace/components";
import { DesktopUi } from "components/mobile/desktop-ui";
import { useEffect } from "react";
import { Platform } from "react-native";
import { useStore } from "store";
import { queryToProps } from "./util";

const isWeb = Platform.OS === "web";

export default function EmbedScreen({ route }) {
  const { user, protocol, url } = route.params;
  let extraProps: Partial<PlayerProps> = {};
  const setSidebarHidden = useStore((state) => state.setSidebarHidden);
  const setSidebarUnhidden = useStore((state) => state.setSidebarUnhidden);
  useEffect(() => {
    setSidebarHidden();
    () => {
      // on unmount, unhide the sidebar
      setSidebarUnhidden();
    };
  }, []);
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  let src = user;
  if (user === "stream") {
    src = url;
  }
  return (
    <LivestreamProvider src={src}>
      <PlayerProvider {...extraProps}>
        <Player src={src} embedded={true} {...extraProps}>
          <DesktopUi />
        </Player>
      </PlayerProvider>
    </LivestreamProvider>
  );
}
