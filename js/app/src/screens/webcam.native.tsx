import { Player } from "components/player/player";

export default function StreamScreen({ route }) {
  return <Player ingest={true} src="live" />;
}
