import { Player } from "components/player/player";

export default function StreamScreen() {
  const params = new URLSearchParams(window.location.search);
  return <Player ingest={true} src="live" {...Object.fromEntries(params)} />;
}
