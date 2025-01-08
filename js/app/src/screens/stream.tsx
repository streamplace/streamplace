import { Player } from "components/player/player";
import { PlayerProps } from "components/player/props";
import { isWeb } from "tamagui";
import { queryToProps } from "./util";

export default function StreamScreen({ route }) {
  const { user, protocol, url } = route.params;
  let extraProps: Partial<PlayerProps> = {};
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  if (user === "stream") {
    return <Player src={url} forceProtocol={protocol} {...extraProps} />;
  }
  return <Player src={user} forceProtocol={protocol} {...extraProps} />;
}
