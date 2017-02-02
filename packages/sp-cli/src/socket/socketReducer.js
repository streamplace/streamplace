
import * as actions from "../constants/actionNames";

export default function socketReducer(state = {}, action) {
  switch (action.type) {

    case actions.COMMAND_SYNC:
      return {...state, devServer: action.config.devServer};

    case actions.COMMAND_SERVE:
      return {...state, port: action.config.port};

  }

  return state;
}
