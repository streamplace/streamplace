
// I made all my reducers one because I was having problems

import {combineReducers} from "redux";

import terminalReducer from "./terminal/terminalReducer";
import watcherReducer from "./watcher/watcherReducer";

export default combineReducers({
  terminal: terminalReducer,
  watcher: watcherReducer,
});

