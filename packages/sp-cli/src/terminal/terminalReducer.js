
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
    },
    watcher: {
      name: "watcher",
      color: [100, 255, 100]
    }
  },
  entries: [],
};

export default function terminal(state = initialState, action) {

}
