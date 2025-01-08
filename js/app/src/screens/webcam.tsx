import { Player } from "components/player/player";
import { queryToProps } from "./util";

export default function StreamScreen() {
  const params = new URLSearchParams(window.location.search);
  return <Player ingest={true} src="live" {...queryToProps(params)} />;
}
