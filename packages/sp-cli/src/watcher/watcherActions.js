
import chokidar from "chokidar";
import fs from "mz/fs";
import {
  WATCHER_READY,
  WATCHER_ADD,
  WATCHER_CHANGE,
  WATCHER_UNLINK,
  WATCHER_LOAD_FILE_SUCCESS,
  WATCHER_LOAD_FILE_ERROR,
} from "../constants/actionNames";

const IGNORED = [
  "node_modules",
  "dist",
];

export const watcherWatch = () => dispatch => {
  chokidar.watch(".", {ignored: IGNORED})
  .on("ready", () => {
    dispatch(watcherReady());
  })
  // Chokidar's funny and calls everything twice, with the stat block the second time. We only
  // care then.
  .on("add", (path, stat) => {
    if (!stat) {
      return;
    }
    dispatch(watcherAdd(path, stat));
  })
  .on("change", (path, stat) => {
    if (!stat) {
      return;
    }
    dispatch(watcherChange(path, stat));
  });
};

export function watcherReady() {
  return {
    type: WATCHER_READY,
  };
}

export const watcherAdd = (path, stat) => dispatch => {
  return Promise.resolve()
  .then(() => {
    dispatch({
      type: WATCHER_ADD,
      path: path,
      stat: stat,
    });
    return watcherLoadFile(path)(dispatch);
  });
};

export const watcherChange = (path, stat) => dispatch => {
  return Promise.resolve()
  .then(() => {
    dispatch({
      type: WATCHER_CHANGE,
      path: path,
      stat: stat,
    });
    return watcherLoadFile(path)(dispatch);
  });
};

export const watcherUnlink = (path, stat) => {
  return {
    type: WATCHER_UNLINK,
    path: path,
    stat: stat,
  };
};

export const watcherLoadFile = (path) => dispatch => {
  return fs.readFile(path, "utf8")
  .then((buf) => {
    return dispatch({
      type: WATCHER_LOAD_FILE_SUCCESS,
      path: path,
      buffer: buf,
    });
  })
  .catch((err) => {
    return dispatch({
      type: WATCHER_LOAD_FILE_ERROR,
      path: path,
      error: err,
    });
  });
};
