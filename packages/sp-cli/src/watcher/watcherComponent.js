
import chokidar from "chokidar";
import {watcherReady, watcherAdd, watcherChange, watcherUnlink} from "./watcherActions";

const IGNORED = [
  "node_modules",
  "dist",
];

export default function watcherComponent(store) {
  chokidar.watch(".", {ignored: IGNORED})
  .on("ready", () => {
    store.dispatch(watcherReady());
  })
  // Chokidar's funny and calls everything twice, with the stat block the second time. We only
  // care then.
  .on("add", (path, stat) => {
    if (!stat) {
      return;
    }
    store.dispatch(watcherAdd(path, stat));
  })
  .on("change", (path, stat) => {
    if (!stat) {
      return;
    }
    store.dispatch(watcherChange(path, stat));
  });
}
