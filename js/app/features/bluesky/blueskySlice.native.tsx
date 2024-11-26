import { OAuthSession } from "@atproto/oauth-client";
import { createAppSlice } from "../../hooks/createSlice";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { Agent } from "@atproto/api";

export interface BlueskyState {
  status: "start" | "loggedIn" | "loggedOut";
  oauthState: null | string;
  oauthSession: null | OAuthSession;
  pdsAgent: null | Agent;
  profiles: { [key: string]: ProfileViewDetailed };
  client: null;
}

const initialState: BlueskyState = {
  status: "start",
  oauthState: null,
  oauthSession: null,
  pdsAgent: null,
  profiles: {},
  client: null,
};

export const blueskySlice = createAppSlice({
  name: "bluesky",
  initialState,
  reducers: (create) => ({
    loadOAuthClient: create.asyncThunk(async (_, { getState }) => {}, {
      pending: (state) => {
        // state.status = "loading";
      },
      fulfilled: (state, action) => {},
      rejected: (_, { error }) => {},
    }),

    login: create.asyncThunk(async (pds: string, thunkAPI) => {}, {
      pending: (state) => {
        // state.status = "loading";
      },
      fulfilled: (state, action) => {
        return state;
      },
      rejected: (state, action) => {
        console.error("login rejected", action.error);
        return {
          ...state,
          profiles: {},
        };
        // state.status = "failed";
      },
    }),

    logout: create.asyncThunk(async (_, thunkAPI) => {}, {
      pending: (state) => {
        // state.status = "loading";
      },
      fulfilled: (state, action) => {},
      rejected: (state) => {},
    }),

    getProfile: create.asyncThunk(async (actor: string, thunkAPI) => {}, {
      pending: (state) => {
        // state.status = "loading";
      },
      fulfilled: (state, action) => {},
      rejected: (state, action) => {},
    }),

    golivePost: create.asyncThunk(
      async (
        {
          nodeUrl,
          signingKey,
          text,
        }: { nodeUrl: string; signingKey: string; text: string },
        thunkAPI,
      ) => {},
      {
        pending: (state) => {
          console.log("golivePost pending");
        },
        fulfilled: (state, action) => {
          console.log("golivePost fulfilled", action.payload);
        },
        rejected: (state, action) => {
          console.error("getProfile rejected", action.error);
          // state.status = "failed";
        },
      },
    ),
  }),

  // You can define your selectors here. These selectors receive the slice
  // state as their first argument.
  selectors: {
    selectOAuthSession: (bluesky) => bluesky.oauthSession,
    selectProfiles: (bluesky) => bluesky.profiles,
    selectUserProfile: (bluesky) => {
      const did = bluesky.oauthSession?.did;
      if (!did) return null;
      return bluesky.profiles[did];
    },
  },
});

// Action creators are generated for each case reducer function.
export const { loadOAuthClient, login, getProfile, logout, golivePost } =
  blueskySlice.actions;

// Selectors returned by `slice.selectors` take the root state as their first argument.
export const { selectOAuthSession, selectProfiles, selectUserProfile } =
  blueskySlice.selectors;
