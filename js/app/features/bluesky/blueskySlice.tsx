import { OAuthSession } from "@atproto/oauth-client";
import { createAppSlice } from "../../hooks/createSlice";
import oauthClient from "./oauthClient";
import { Agent } from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";

export interface BlueskySliceState {
  status: "start" | "loggedIn" | "loggedOut";
  oauthState: null | string;
  oauthSession: null | OAuthSession;
  pdsAgent: null | Agent;
  profiles: { [key: string]: ProfileViewDetailed };
  // client: null | BrowserOAuthClient;
}

const initialState: BlueskySliceState = {
  status: "start",
  oauthState: null,
  oauthSession: null,
  pdsAgent: null,
  profiles: {},
};

export const blueskySlice = createAppSlice({
  name: "bluesky",
  initialState,
  reducers: (create) => ({
    loadOAuthClient: create.asyncThunk(
      async () => {
        return oauthClient.init();
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          if (action.payload && "session" in action.payload) {
            return {
              ...state,
              oauthSession: action.payload.session,
              pdsAgent: new Agent(action.payload.session),
            };
          }
          return state;
        },
        rejected: (state) => {
          console.error("loadOAuthClient rejected");
          // state.status = "failed";
        },
      },
    ),

    login: create.asyncThunk(
      async (pds: string) => {
        return await oauthClient.authorize(pds);
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          document.location.href = action.payload.toString();
          return state;
        },
        rejected: (state) => {
          console.error("login rejected");
          // state.status = "failed";
        },
      },
    ),

    logout: create.asyncThunk(
      async (_, thunkAPI) => {
        const { bluesky } = thunkAPI.getState() as {
          bluesky: BlueskySliceState;
        };
        if (!bluesky.oauthSession) {
          throw new Error("No oauth session");
        }
        return bluesky.oauthSession.signOut();
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            oauthSession: null,
            pdsAgent: null,
          };
        },
        rejected: (state) => {
          console.error("logout rejected");
          // state.status = "failed";
        },
      },
    ),

    getProfile: create.asyncThunk(
      async (actor: string, thunkAPI) => {
        const { bluesky } = thunkAPI.getState() as {
          bluesky: BlueskySliceState;
        };
        if (!bluesky.pdsAgent) {
          throw new Error("No agent");
        }
        return await bluesky.pdsAgent.getProfile({
          actor: actor,
        });
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            profiles: {
              ...state.profiles,
              [action.meta.arg]: action.payload.data,
            },
          };
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
export const { loadOAuthClient, login, getProfile, logout } =
  blueskySlice.actions;

// Selectors returned by `slice.selectors` take the root state as their first argument.
export const { selectOAuthSession, selectProfiles, selectUserProfile } =
  blueskySlice.selectors;
