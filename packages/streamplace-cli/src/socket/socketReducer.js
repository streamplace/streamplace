import * as actions from "../constants/actionNames";

const initialState = {
  connected: false,
  ws: null
};

export default function socketReducer(state = initialState, action) {
  switch (action.type) {
    case actions.COMMAND_SYNC:
      return { ...state, devServer: action.config.devServer };

    case actions.COMMAND_SERVE:
      return { ...state, port: action.config.port };

    case actions.SOCKET_CONNECT_SUCCESS:
      return { ...state, connected: true, ws: action.ws };

    case actions.SOCKET_CLOSE:
      return { ...state, connected: false, ws: null };
  }

  return state;
}
