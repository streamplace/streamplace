import Livestream from "components/livestream/livestream";
import { PlayerProps } from "components/player/props";
import { FullscreenProvider } from "contexts/FullscreenContext";
import { isWeb } from "tamagui";
import { queryToProps } from "./util";

export default function StreamScreen({ route }) {
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
    <FullscreenProvider>
      <Livestream src={src} {...extraProps} />
    </FullscreenProvider>
  );
}
