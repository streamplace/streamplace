import { OAuthSession } from "@atproto/oauth-client";
import { createAppSlice } from "../../hooks/createSlice";
import oauthClient from "./oauthClient";
import { Agent } from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";

export interface BlueskyState {
  status: "start" | "loggedIn" | "loggedOut";
  oauthState: null | string;
  oauthSession: null | OAuthSession;
  pdsAgent: null | Agent;
  profiles: { [key: string]: ProfileViewDetailed };
  // client: null | BrowserOAuthClient;
}

const initialState: BlueskyState = {
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
          return {
            ...state,
            profiles: {},
          };
          // state.status = "failed";
        },
      },
    ),

    logout: create.asyncThunk(
      async (_, thunkAPI) => {
        const { bluesky } = thunkAPI.getState() as {
          bluesky: BlueskyState;
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
          bluesky: BlueskyState;
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

    golivePost: create.asyncThunk(
      async (
        {
          nodeUrl,
          signingKey,
          text,
        }: { nodeUrl: string; signingKey: string; text: string },
        thunkAPI,
      ) => {
        const { bluesky } = thunkAPI.getState() as {
          bluesky: BlueskyState;
        };
        if (!bluesky.pdsAgent) {
          throw new Error("No agent");
        }
        const did = bluesky.oauthSession?.did;
        if (!did) {
          throw new Error("No DID");
        }
        const profile = bluesky.profiles[did];
        if (!profile) {
          throw new Error("No profile");
        }
        const u = new URL(nodeUrl);
        const params = new URLSearchParams({
          key: signingKey,
          did: did,
          time: new Date().toISOString(),
        });
        const linkUrl = `${u.protocol}//${u.host}/${profile.handle}?${params.toString()}`;
        const prefix = `🔴 LIVE `;
        const textUrl = `${u.protocol}//${u.host}/${profile.handle}`;
        const suffix = ` ${text}`;
        const content = prefix + textUrl + suffix;
        const facets = [
          {
            index: {
              // idk why it's off by two but it's static so let's just rock it
              byteStart: prefix.length + 2,
              byteEnd: prefix.length + textUrl.length + 2,
            },
            features: [
              {
                $type: "app.bsky.richtext.facet#link",
                uri: linkUrl,
              },
            ],
          },
        ];
        const record = {
          text: content,
          "tv.aquareum.key": signingKey,
          facets,
        };
        return await bluesky.pdsAgent.post(record);
      },
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
