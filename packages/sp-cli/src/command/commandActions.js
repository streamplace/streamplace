
import {COMMAND_SYNC} from "../constants/actionNames";
import {watcherWatch} from "../watcher/watcherActions";

export const commandSync = () => dispatch => {
  dispatch({
    type: COMMAND_SYNC
  });
  dispatch(watcherWatch());
};
