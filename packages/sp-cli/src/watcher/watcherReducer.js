
import * as actions from "../constants/actionNames";

const initialState = {
  ready: false,
  files: {},
};

function upsertFile(state, action) {
  const {path} = action;
  const oldFile = state.files[path] || {};
  const newFile = {
    ...oldFile,
  };
  if (action.stat) {
    newFile.stat = action.stat;
  }
  if (action.buffer) {
    newFile.buffer = action.buffer;
  }
  if (newFile.uploaded === undefined) {
    newFile.uploaded = false;
  }
  return {
    ...state,
    files: {
      ...state.files,
      [path]: newFile
    }
  };
}

export default function watcherReducer(state = initialState, action) {
  switch (action.type) {

    case actions.WATCHER_READY:
      return {
        ...state,
        ready: true
      };

    case actions.WATCHER_ADD:
      return upsertFile(state, action);

    case actions.WATCHER_CHANGE:
      return upsertFile(state, action);

  }

  return state;
}
