
import * as actions from "../constants/actionNames";

function addEntry(state, category, text) {
  return {
    ...state,
    entries: [
      ...state.entries,
      {category, text}
    ],
  };
}

const initialState = {
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
    },
    socket: {
      name: "socket",
      color: [100, 100, 255]
    },
  },
  bottomBar: false,
  entries: [],
};

export default function terminalReducer(state = initialState, action) {
  switch (action.type) {

    case actions.COMMAND_SYNC:
      return {...state, bottomBar: true};

    case actions.WATCHER_READY:
      return addEntry(state, "watcher", "Watcher ready!");

    case actions.WATCHER_ADD:
      return addEntry(state, "watcher", `File added: ${action.path}`);

    case actions.WATCHER_CHANGE:
      return addEntry(state, "watcher", `File changed: ${action.path}`);

    case actions.WATCHER_UNLINK:
      return addEntry(state, "watcher", `File deleted: ${action.path}`);

    case actions.WATCHER_LOAD_FILE_SUCCESS:
      return addEntry(state, "watcher", `File loaded: ${action.path}`);

    case actions.WATCHER_LOAD_FILE_ERROR:
      return addEntry(state, "watcher", `Error loading file ${action.path}: ${JSON.stringify(action)}`);

    case actions.SOCKET_LISTEN_SUCCESS:
      return addEntry(state, "socket", `Streamplace dev server listening on port ${action.port}`);

    case actions.SOCKET_CONNECT:
      return addEntry(state, "socket", `Attempting connection to ${action.server}`);

    case actions.SOCKET_CONNECT_SUCCESS:
      state = addEntry(state, "socket", `Opened connection ${action.server}`);
      return {
        ...state,
        status: {
          color: [0, 255, 0],
          text: "Connected",
        },
      };

    case actions.SOCKET_ERROR:
      state = addEntry(state, "socket", `Socket error ${action.error}`);
      return {
        ...state,
        status: initialState.status,
      };

    case actions.SOCKET_CLOSE:
      state = addEntry(state, "socket", `Connection to ${action.server} closed`);
      return {
        ...state,
        status: initialState.status,
      };
  }

  return state;
}
