import { Player } from "components/player/player";
import { PlayerProps } from "components/player/props";
import { isWeb } from "tamagui";
import { queryToProps } from "./util";

const StreamScreenProps = ({
  route,
  ...props
}: Partial<PlayerProps> & { route: any }) => {
  const { user, protocol, url } = route.params;
  let extraProps: Partial<PlayerProps> = {};
  if (isWeb) {
    extraProps = queryToProps(new URLSearchParams(window.location.search));
  }
  if (user === "stream") {
    return (
      <Player src={url} forceProtocol={protocol} {...props} {...extraProps} />
    );
  }
  return (
    <Player src={user} forceProtocol={protocol} {...props} {...extraProps} />
  );
};

export default function StreamScreen({ route }) {
  return <StreamScreenProps route={route} />;
}

export function EmbedStreamScreen({ route }) {
  return <StreamScreenProps route={route} embed={true} />;
}
