import { storage } from "@streamplace/components";
import { BlueskyState } from "features/bluesky/blueskyTypes";
import { PlaceStreamLivestream, PlaceStreamSegment } from "streamplace";
import { isWeb } from "tamagui";
import { createAppSlice } from "../../hooks/createSlice";

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
  viewers: number;
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
    segments: PlaceStreamLivestream.LivestreamView[];
    error: string | null;
    loading: boolean;
    firstRequest: boolean;
  };
  mySegments: PlaceStreamSegment.SegmentView[];
  userMuted: boolean | null;
  chatWarned: boolean;
}

const initialState: StreamplaceState = {
  url: DEFAULT_URL,
  identity: null,
  initialized: false,
  recentSegments: {
    segments: [],
    error: null,
    loading: false,
    firstRequest: true,
  },
  mySegments: [],
  userMuted: null,
  chatWarned: false,
};

const USER_MUTED_KEY = "streamplaceUserMuted";
const URL_KEY = "streamplaceUrl";
const CHAT_WARNING_KEY = "streamplaceChatWarning2";

export const streamplaceSlice = createAppSlice({
  name: "streamplace",
  initialState,
  reducers: (create) => ({
    initialize: create.asyncThunk(
      async (_, { getState }) => {
        let [url, userMutedStr, chatWarningStr] = await Promise.all([
          storage.getItem(URL_KEY),
          storage.getItem(USER_MUTED_KEY),
          storage.getItem(CHAT_WARNING_KEY),
        ]);
        if (!url) {
          url = DEFAULT_URL;
        }
        let userMuted: boolean | null = null;
        console.log("userMutedStr", userMutedStr);
        if (typeof userMutedStr === "string") {
          userMuted = userMutedStr === "true";
        } else {
          userMuted = null;
        }
        let chatWarned: boolean = false;
        if (typeof chatWarningStr === "string") {
          chatWarned = chatWarningStr === "true";
        }
        return { url, userMuted, chatWarned };
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          const { url, userMuted, chatWarned } = action.payload;
          return {
            ...state,
            url,
            userMuted,
            initialized: true,
            chatWarned,
          };
        },
        rejected: (_, { error }) => {
          // state.status = "failed";
        },
      },
    ),

    setURL: create.reducer((state, action: { payload: string }) => {
      console.log("setURL", action);
      storage.setItem(URL_KEY, action.payload).catch((err) => {
        console.error("setURL error", err);
      });
      return {
        ...state,
        url: action.payload,
      };
    }),

    userMute: create.reducer((state, action: { payload: boolean }) => {
      storage
        .setItem(USER_MUTED_KEY, JSON.stringify(action.payload))
        .catch((err) => {
          console.error("userMute error", err);
        });
      return {
        ...state,
        userMuted: action.payload,
      };
    }),

    chatWarn: create.reducer((state, action: { payload: boolean }) => {
      storage
        .setItem(CHAT_WARNING_KEY, JSON.stringify(action.payload))
        .catch((err) => {
          console.error("chatWarn error", err);
        });
      return {
        ...state,
        chatWarned: action.payload,
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

    pollMySegments: create.asyncThunk(
      async (_, { getState, dispatch }) => {
        const { streamplace } = getState() as {
          streamplace: StreamplaceState;
        };
        const { bluesky } = getState() as {
          bluesky: BlueskyState;
        };

        if (!bluesky.pdsAgent) {
          throw new Error("no pdsAgent");
        }
        if (!bluesky.oauthSession) {
          throw new Error("no oauthSession");
        }
        return await bluesky.pdsAgent.place.stream.live.getSegments({
          userDID: bluesky.oauthSession?.did ?? "",
        });
      },
      {
        pending: (state) => {
          return {
            ...state,
          };
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            mySegments: action.payload.data.segments ?? [],
          };
        },
        rejected: (state, err) => {
          // console.error("pollMySegments rejected", err);
          return {
            ...state,
          };
        },
      },
    ),
  }),

  selectors: {
    selectStreamplace: (streamplace) => streamplace,
    selectUrl: (streamplace) => streamplace.url,
    selectInitialized: (streamplace) => streamplace.initialized,
    selectRecentSegments: (streamplace) => streamplace.recentSegments,
    selectMySegments: (streamplace) => streamplace.mySegments,
    selectUserMuted: (streamplace) => streamplace.userMuted,
    selectChatWarned: (streamplace) => streamplace.chatWarned,
  },
});

// Action creators are generated for each case reducer function.
export const {
  getIdentity,
  setURL,
  initialize,
  pollMySegments,
  userMute,
  chatWarn,
} = streamplaceSlice.actions;
export const {
  selectStreamplace,
  selectMySegments,
  selectUserMuted,
  selectChatWarned,
  selectUrl,
  selectInitialized,
} = streamplaceSlice.selectors;
