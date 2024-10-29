import { Player } from "components/player/player";

export default function StreamScreen({ route }) {
  const { user } = route.params;
  return <Player src={user} />;
}
