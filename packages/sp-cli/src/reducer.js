
// I made all my reducers one because I was having problems

import * as actions from "./constants/actionNames";

const initialState = {
  terminal: {
    command: null,
    title: {
      color: [255, 207, 0],
      text: "Streamplace",
    },
    status: {
      color: [255, 0, 0],
      text: "Disconnected"
    },
    categories: {
      debug: {
        name: "debug",
        color: [0, 255, 255]
      },
      watcher: {
        name: "watcher",
        color: [100, 255, 100]
      }
    },
    entries: [],
  },
  watcher: {
    ready: false,
    files: {},
  },
};

function addEntry(state, category, text) {
  return {
    ...state,
    terminal: {
      ...state.terminal,
      entries: [
        ...state.terminal.entries,
        {category, text}
      ],
    }
  };
}

function upsertFile(state, action) {
  const {path} = action;
  const oldFile = state.watcher.files[path] || {};
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
    watcher: {
      ...state.watcher,
      files: {
        ...state.watcher.files,
        [path]: newFile
      }
    }
  };
}

/*eslint indent: ["error", 2, { "SwitchCase": 1 }]*/
export default function rootReducer(state = initialState, action) {
  switch (action.type) {

    case actions.TERMINAL_COMMAND:
      return {
        ...state,
        terminal: {
          ...state.terminal,
          command: action.command,
        }
      };

    case actions.WATCHER_READY:
      state = addEntry(state, "watcher", "Watcher ready!");
      return {
        ...state,
        watcher: {
          ...state.watcher,
          ready: true
        }
      };

    case actions.WATCHER_ADD:
      // Only log after the initial ready
      if (state.watcher.ready === true) {
        state = addEntry(state, "watcher", `File added: ${action.path}`);
      }
      if (state.watcher.files[action.path]) {
        state = addEntry(state, "debug", `We got an added event for ${action.path} but we already know about it, weird.`);
      }
      else {
        state = upsertFile(state, action);
      }
      return state;

    case actions.WATCHER_CHANGE:
      state = addEntry(state, "watcher", `File changed: ${action.path}`);
      return upsertFile(state, action);

    case actions.WATCHER_UNLINK:
      return addEntry(state, "watcher", `File deleted: ${action.path}`);

    case actions.WATCHER_LOAD_FILE_SUCCESS:
      return addEntry(state, "watcher", `File loaded: ${action.path}`);


    case actions.WATCHER_LOAD_FILE_ERROR:
      return addEntry(state, "watcher", `Error loading file ${action.path}: ${JSON.stringify(action)}`);

  }
  return state;
}
