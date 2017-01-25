
import * as actions from "../constants/actionNames";

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
    }
  },
  entries: [],
};

function addEntry(state, category, text) {
  return {
    ...state,
    entries: [
      ...state.entries,
      {category, text}
    ],
  };
}

export default function terminal(state = initialState, action) {
  switch (action.type) {
  case actions.STARTUP:
    return addEntry(state, "debug", "Starting up...");
  }

  return state;
}
