import { isWeb } from "tamagui";
import { createAppSlice } from "../../hooks/createSlice";
import Storage from "../../storage";
import { BlueskyState } from "features/bluesky/blueskyTypes";
import { SegmentView } from "lexicons/types/place/stream/segment";
import { LivestreamView } from "lexicons/types/place/stream/livestream";
import { StreamplaceAgent } from "features/bluesky/agent";

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
    segments: LivestreamView[];
    error: string | null;
    loading: boolean;
    firstRequest: boolean;
  };
  mySegments: SegmentView[];
  telemetry: boolean | null;
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
  telemetry: null,
  userMuted: null,
  chatWarned: false,
};

const USER_MUTED_KEY = "streamplaceUserMuted";
const TELEMETRY_KEY = "streamplaceTelemetry";
const URL_KEY = "streamplaceUrl";
const CHAT_WARNING_KEY = "streamplaceChatWarning2";

export const streamplaceSlice = createAppSlice({
  name: "streamplace",
  initialState,
  reducers: (create) => ({
    initialize: create.asyncThunk(
      async (_, { getState }) => {
        let [url, telemetryStr, userMutedStr, chatWarningStr] =
          await Promise.all([
            Storage.getItem(URL_KEY),
            Storage.getItem(TELEMETRY_KEY),
            Storage.getItem(USER_MUTED_KEY),
            Storage.getItem(CHAT_WARNING_KEY),
          ]);
        if (!url) {
          url = DEFAULT_URL;
        }
        let telemetry: boolean | null = null;
        if (typeof telemetryStr === "string") {
          telemetry = JSON.parse(telemetryStr);
        } else {
          telemetry = null;
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
        return { url, telemetry, userMuted, chatWarned };
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          const { url, telemetry, userMuted, chatWarned } = action.payload;
          return {
            ...state,
            url,
            telemetry,
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
      Storage.setItem(URL_KEY, action.payload).catch((err) => {
        console.error("setURL error", err);
      });
      return {
        ...state,
        url: action.payload,
      };
    }),

    telemetryOpt: create.reducer((state, action: { payload: boolean }) => {
      Storage.setItem(TELEMETRY_KEY, JSON.stringify(action.payload)).catch(
        (err) => {
          console.error("telemetryOpt error", err);
        },
      );
      return {
        ...state,
        telemetry: action.payload,
      };
    }),

    userMute: create.reducer((state, action: { payload: boolean }) => {
      Storage.setItem(USER_MUTED_KEY, JSON.stringify(action.payload)).catch(
        (err) => {
          console.error("userMute error", err);
        },
      );
      return {
        ...state,
        userMuted: action.payload,
      };
    }),

    chatWarn: create.reducer((state, action: { payload: boolean }) => {
      Storage.setItem(CHAT_WARNING_KEY, JSON.stringify(action.payload)).catch(
        (err) => {
          console.error("chatWarn error", err);
        },
      );
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

    pollSegments: create.asyncThunk(
      async (_, { getState, dispatch }) => {
        const { streamplace } = getState() as {
          streamplace: StreamplaceState;
        };
        const { bluesky } = getState() as {
          bluesky: BlueskyState;
        };

        let agent = bluesky.pdsAgent;

        if (!agent) {
          agent = new StreamplaceAgent(streamplace.url);
        }

        let users = await agent.place.stream.live.getLiveUsers();

        return users.data.streams;
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
              segments: action.payload || [],
              loading: false,
              error: null,
              firstRequest: false,
            },
          };
        },
        rejected: (state, err) => {
          console.error("pollSegments rejected", err);
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
        return await bluesky.pdsAgent.place.stream.live.getSegments({
          userDID: bluesky.oauthSession?.did,
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
          console.error("pollMySegments rejected", err);
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
    selectRecentSegments: (streamplace) => streamplace.recentSegments,
    selectMySegments: (streamplace) => streamplace.mySegments,
    selectTelemetry: (streamplace) => streamplace.telemetry,
    selectUserMuted: (streamplace) => streamplace.userMuted,
    selectChatWarned: (streamplace) => streamplace.chatWarned,
  },
});

// Action creators are generated for each case reducer function.
export const {
  getIdentity,
  setURL,
  initialize,
  pollSegments,
  pollMySegments,
  telemetryOpt,
  userMute,
  chatWarn,
} = streamplaceSlice.actions;
export const {
  selectStreamplace,
  selectRecentSegments,
  selectMySegments,
  selectTelemetry,
  selectUserMuted,
  selectChatWarned,
  selectUrl,
} = streamplaceSlice.selectors;
