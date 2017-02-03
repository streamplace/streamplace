
import {COMMAND_SYNC, COMMAND_SERVE} from "../constants/actionNames";
import {watcherWatch} from "../watcher/watcherActions";
import {socketConnect} from "../socket/socketActions";
import {serverListen} from "../server/serverActions";

export const commandSync = (config) => dispatch => {
  dispatch({
    type: COMMAND_SYNC,
    config: config,
  });
  dispatch(watcherWatch());
  dispatch(socketConnect(config.devServer));
};

export const commandServe = (config) => dispatch => {
  dispatch({
    type: COMMAND_SERVE,
    config: config,
  });
  dispatch(serverListen());
};
