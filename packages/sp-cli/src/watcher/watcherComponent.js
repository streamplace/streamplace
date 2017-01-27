
import {watcherWatch} from "./watcherActions";
import {SYNC} from "../constants/commands";

export default function watcherComponent(store) {
  const state = store.getState();
  if (state.terminal.command === SYNC) {
    store.dispatch(watcherWatch());
  }
}
