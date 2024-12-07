import { Player } from "components/player/player";

export default function StreamScreen({ route }) {
  console.log(route);
  const { user, protocol } = route.params;
  return <Player src={user} forceProtocol={protocol} />;
}
