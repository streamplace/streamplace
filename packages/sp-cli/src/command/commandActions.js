
import {COMMAND_SYNC, COMMAND_SERVE} from "../constants/actionNames";
import {watcherWatch} from "../watcher/watcherActions";
import {socketListen, socketConnect} from "../socket/socketActions";

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
  dispatch(socketListen());
};
