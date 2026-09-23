import path from "path";

// Where global-setup saves the post-server-setup browser state (localStorage
// holds the custom-node URL, persisted by redux-persist) for the flows to reuse.
export const STORAGE_STATE = path.join(__dirname, ".playwright", "state.json");
