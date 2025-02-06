import { createAppSlice } from "../../hooks/createSlice";
import { isWeb } from "tamagui";
import Storage from "../../storage";

let DEFAULT_URL = process.env.EXPO_PUBLIC_STREAMPLACE_URL as string;
if (isWeb && process.env.EXPO_PUBLIC_WEB_TRY_LOCAL === "true") {
  try {
    DEFAULT_URL = `${window.location.protocol}//${window.location.host}`;
  } catch (err) {
    // Oh well, fall back to hardcoded.
  }
}
export { DEFAULT_URL };

export type Segment = {
  id: string;
  repoDID: string;
  signingKeyDID: string;
  startTime: string;
  repo: Repo;
};

export type Repo = {
  did: string;
  handle: string;
  pds: string;
  version: string;
  rootCid: string;
};

export interface Identity {
  id: string;
  handle?: string;
  did?: string;
}

export interface StreamplaceState {
  url: string;
  identity: Identity | null;
  initialized: boolean;
  recentSegments: {
    segments: Segment[];
    error: string | null;
    loading: boolean;
  };
}

const initialState: StreamplaceState = {
  url: DEFAULT_URL,
  identity: null,
  initialized: false,
  recentSegments: {
    segments: [],
    error: null,
    loading: false,
  },
};

export const streamplaceSlice = createAppSlice({
  name: "streamplace",
  initialState,
  reducers: (create) => ({
    initialize: create.asyncThunk(
      async (_, { getState }) => {
        let url = await Storage.getItem("streamplaceUrl");
        if (!url) {
          url = DEFAULT_URL;
        }
        return url;
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          const url = action.payload;
          return {
            ...state,
            url,
            initialized: true,
          };
        },
        rejected: (_, { error }) => {
          // state.status = "failed";
        },
      },
    ),

    setURL: create.reducer((state, action: { payload: string }) => {
      Storage.setItem("streamplaceUrl", action.payload).catch((err) => {
        console.error("setURL error", err);
      });
      return {
        ...state,
        url: action.payload,
      };
    }),

    getIdentity: create.asyncThunk(
      async (_, { getState }) => {
        const { streamplace } = getState() as {
          streamplace: StreamplaceState;
        };
        const res = await fetch(`${streamplace.url}/api/identity`);
        return await res.json();
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            identity: action.payload,
          };
        },
        rejected: (state) => {
          console.error("loadOAuthClient rejected");
          // state.status = "failed";
        },
      },
    ),

    pollSegments: create.asyncThunk(
      async (_, { getState, dispatch }) => {
        const { streamplace } = getState() as {
          streamplace: StreamplaceState;
        };
        const res = await fetch(`${streamplace.url}/api/live-users`);
        if (!res.ok) {
          const text = await res.text();
          throw new Error(`http ${res.status} ${text}`);
        }
        const data = await res.json();
        if (!Array.isArray(data)) {
          throw new Error("got non-array back from /api/live-users");
        }

        return data;
      },
      {
        pending: (state) => {
          return {
            ...state,
            recentSegments: {
              ...state.recentSegments,
              loading: true,
            },
          };
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            recentSegments: {
              ...state.recentSegments,
              segments: action.payload,
              loading: false,
              error: null,
            },
          };
        },
        rejected: (state, err) => {
          return {
            ...state,
            recentSegments: {
              ...state.recentSegments,
              error: err.error.message ?? null,
              loading: false,
            },
          };
        },
      },
    ),
  }),

  selectors: {
    selectStreamplace: (streamplace) => streamplace,
    selectRecentSegments: (streamplace) => streamplace.recentSegments,
  },
});

// Action creators are generated for each case reducer function.
export const { getIdentity, setURL, initialize, pollSegments } =
  streamplaceSlice.actions;
export const { selectStreamplace, selectRecentSegments } =
  streamplaceSlice.selectors;
