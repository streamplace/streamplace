import { PlayerProps } from "components/player/props";
import { isWeb } from "tamagui";
import { queryToProps } from "./util";
import { Player } from "components";

export default function EmbedScreen({ route }) {
  const { user, protocol, url } = route.params;
  let extraProps: Partial<PlayerProps> = {};
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  let src = user;
  if (user === "stream") {
    src = url;
  }
  return <Player src={src} embedded={true} {...extraProps} />;
}
