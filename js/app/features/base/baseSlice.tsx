import { storage } from "@streamplace/components";
import { createAppSlice } from "../../hooks/createSlice";
export const STORED_KEY_KEY = "storedKey";
export const DID_KEY = "did";

export interface StreamKey {
  privateKey: string;
  did: string;
  address: string;
}

export interface BaseState {
  hydrated: boolean;
}

const initialState: BaseState = {
  hydrated: false,
};

export const baseSlice = createAppSlice({
  name: "base",
  initialState,
  reducers: (create) => ({
    hydrate: create.asyncThunk(
      async () => {
        let storedKey: StreamKey | null = null;
        // Async operation would go here
        try {
          const storedKeyStr = await storage.getItem(STORED_KEY_KEY);
          if (storedKeyStr) {
            storedKey = JSON.parse(storedKeyStr);
          }
        } catch (e) {
          // we don't have one i guess
        }
        return { storedKey };
      },
      {
        pending: (state) => {
          state.hydrated = false;
        },
        fulfilled: (state) => {
          state.hydrated = true;
        },
        rejected: (state) => {
          state.hydrated = false;
        },
      },
    ),
  }),
  selectors: {
    selectHydrated: (state) => state.hydrated,
  },
});

export const { hydrate } = baseSlice.actions;
export const { selectHydrated } = baseSlice.selectors;
