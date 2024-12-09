import { Player } from "components/player/player";

export default function StreamScreen({ route }) {
  const { user, protocol, url } = route.params;
  if (user === "stream") {
    return <Player src={url} forceProtocol={protocol} />;
  }
  return <Player src={user} forceProtocol={protocol} />;
}
