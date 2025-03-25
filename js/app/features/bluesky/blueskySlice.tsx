import { Agent, AppBskyFeedPost, BlobRef } from "@atproto/api";
import { StreamplaceState } from "features/streamplace/streamplaceSlice";
import { openLoginLink } from "features/platform/platformSlice";
import Storage from "storage";
import { createAppSlice } from "../../hooks/createSlice";
import createOAuthClient from "./oauthClient";
import { Secp256k1Keypair, bytesToMultibase } from "@atproto/crypto";
import { privateKeyToAccount } from "viem/accounts";
import { hydrate, STORED_KEY_KEY } from "features/base/baseSlice";
import { isWeb } from "tamagui";
import { PlaceStreamKey, PlaceStreamLivestream } from "lexicons";
import { BlueskyState } from "./blueskyTypes";
import { LivestreamViewHydrated } from "features/player/playerSlice";
import { ProfileViewDetailed } from "@atproto/api/src/client/types/app/bsky/actor/defs";

const initialState: BlueskyState = {
  status: "start",
  oauthState: null,
  oauthSession: null,
  pdsAgent: null,
  profiles: {},
  client: null,
  login: {
    loading: false,
    error: null,
  },
  pds: {
    url: "bsky.social",
    loading: false,
    error: null,
  },
  newKey: null,
  storedKey: null,
  newLivestream: null,
};

const uploadThumbnail = async (
  handle: string,
  u: URL,
  pdsAgent: Agent,
  profile: ProfileViewDetailed,
) => {
  // download the thumbnail image and upload it to the pds IF POSSIBLE
  const thumbnailRes = await fetch(
    `${u.protocol}//${u.host}/api/playback/${profile.handle}/stream.png`,
  );
  if (!thumbnailRes.ok) {
    throw new Error(`failed to fetch thumbnail (http ${thumbnailRes.status})`);
  }
  const thumbnailBlob = await thumbnailRes.blob();
  const thumbnail = await pdsAgent.uploadBlob(thumbnailBlob);
  if (!thumbnail.success) {
    throw new Error("failed to upload thumbnail");
  }
  return thumbnail.data.blob;
};

// clear atproto login query params from url
const clearQueryParams = () => {
  if (!isWeb) {
    return;
  }
  const u = new URL(document.location.href);
  const params = new URLSearchParams(u.search);
  if (u.search === "") {
    return;
  }
  params.delete("iss");
  params.delete("state");
  params.delete("code");
  u.search = params.toString();
  window.history.replaceState(null, "", u.toString());
};

export const blueskySlice = createAppSlice({
  name: "bluesky",
  initialState,
  extraReducers: (builder) => {
    builder.addCase(hydrate.fulfilled, (state, action) => {
      return {
        ...state,
        storedKey: action.payload.storedKey,
      };
    });
  },
  reducers: (create) => ({
    loadOAuthClient: create.asyncThunk(
      async (_, { getState }) => {
        const { streamplace } = getState() as { streamplace: StreamplaceState };
        const client = await createOAuthClient(streamplace.url);
        let initResult = await client.init();
        return { client, initResult };
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          const { client, initResult } = action.payload;
          console.log("loadOAuthClient fulfilled", action.payload);
          // sometimes the codes don't get removed from the url properly? so we do so here.
          // const u = new URL(document.location.href);
          // u.search = "";
          // window.history.replaceState(null, "", u.toString());
          if (initResult && "session" in initResult) {
            return {
              ...state,
              client: client,
              oauthSession: initResult.session as any,
              pdsAgent: new Agent(initResult.session),
            };
          }
          return {
            ...state,
            status: "loggedOut",
            client: client,
          };
        },
        rejected: (state, { error }) => {
          return {
            ...state,
            status: "loggedOut",
          };
        },
      },
    ),

    login: create.asyncThunk(
      async (pds: string, thunkAPI) => {
        let { bluesky } = thunkAPI.getState() as {
          bluesky: BlueskyState;
        };
        await thunkAPI.dispatch(loadOAuthClient());
        ({ bluesky } = thunkAPI.getState() as {
          bluesky: BlueskyState;
        });
        if (!bluesky.client) {
          throw new Error("No client");
        }
        const u = await bluesky.client.authorize(pds);
        thunkAPI.dispatch(openLoginLink(u.toString()));
        // cheeky 500ms delay so you don't see the text flash back
        await new Promise((resolve) => setTimeout(resolve, 500));
      },
      {
        pending: (state) => {
          return {
            ...state,
            login: {
              loading: true,
              error: null,
            },
          };
        },
        fulfilled: (state, action) => {
          // document.location.href = action.payload.toString();
          return {
            ...state,
            login: {
              loading: false,
              error: null,
            },
          };
        },
        rejected: (state, action) => {
          return {
            ...state,
            login: {
              loading: false,
              error: action.error?.message ?? null,
            },
          };
          // state.status = "failed";
        },
      },
    ),

    logout: create.asyncThunk(
      async (_, thunkAPI) => {
        await Storage.removeItem("did");
        await Storage.removeItem(STORED_KEY_KEY);
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
          clearQueryParams();
          return {
            ...state,
            status: "loggedIn",
            profiles: {
              ...state.profiles,
              [action.meta.arg]: action.payload.data,
            },
          };
        },
        rejected: (state, action) => {
          clearQueryParams();
          console.error("getProfile rejected", action.error);
          // state.status = "failed";
        },
      },
    ),

    oauthCallback: create.asyncThunk(
      async (url: string, thunkAPI) => {
        console.log("oauthCallback", url);
        if (!url.includes("?")) {
          throw new Error("No query params");
        }
        const params = new URLSearchParams(url.split("?")[1]);
        if (!(params.has("code") && params.has("state") && params.has("iss"))) {
          throw new Error("Missing params, got: " + url);
        }
        const { bluesky } = thunkAPI.getState() as {
          bluesky: BlueskyState;
        };
        if (!bluesky.client) {
          throw new Error("No client");
        }
        try {
          const ret = await bluesky.client.callback(params);
          await Storage.setItem("did", ret.session.did);

          return ret.session as any;
        } catch (e) {
          let message = e.message;
          while (e.cause) {
            message = `${message}: ${e.cause.message}`;
            e = e.cause;
          }
          console.error("oauthCallback error", message);
          throw e;
        }
      },

      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          console.log("oauthCallback fulfilled", action.payload);
          return {
            ...state,
            oauthSession: action.payload as any,
            pdsAgent: new Agent(action.payload) as any,
          };
        },
        rejected: (state, action) => {
          console.error("oauthCallback rejected", action.error);
        },
      },
    ),

    golivePost: create.asyncThunk(
      async (
        { text, now }: { text: string; now: Date },
        thunkAPI,
      ): Promise<{
        uri: string;
        cid: string;
      }> => {
        const { bluesky, streamplace } = thunkAPI.getState() as {
          bluesky: BlueskyState;
          streamplace: StreamplaceState;
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
        const u = new URL(streamplace.url);
        const params = new URLSearchParams({
          did: did,
          time: new Date().toISOString(),
        });

        let thumbnail: BlobRef | null = null;
        try {
          thumbnail = await uploadThumbnail(
            profile.handle,
            u,
            bluesky.pdsAgent,
            profile,
          );
        } catch (e) {
          console.error("uploadThumbnail error", e);
        }

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
        const record: AppBskyFeedPost.Record = {
          text: content,
          "place.stream.livestream": {
            url: linkUrl,
            title: text,
          },
          facets,
          createdAt: now.toISOString(),
        };
        if (thumbnail) {
          record.embed = {
            $type: "app.bsky.embed.external",
            external: {
              description: text,
              thumb: thumbnail,
              title: `@${profile.handle} is 🔴LIVE on ${u.host}!`,
              uri: linkUrl,
            },
          };
        }
        console.log("golivePost record", record);
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
          console.error("golivePost rejected", action.error);
          // state.status = "failed";
        },
      },
    ),

    chatPost: create.asyncThunk(
      async (
        {
          text,
          livestream,
        }: { text: string; livestream: LivestreamViewHydrated },
        thunkAPI,
      ) => {
        const { bluesky, streamplace } = thunkAPI.getState() as {
          bluesky: BlueskyState;
          streamplace: StreamplaceState;
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
        if (!livestream.record.post) {
          throw new Error("No post");
        }
        const record: AppBskyFeedPost.Record = {
          text: text,
          createdAt: new Date().toISOString(),
          reply: {
            root: {
              cid: livestream.record.post.cid,
              uri: livestream.record.post.uri,
            },
            parent: {
              cid: livestream.record.post.cid,
              uri: livestream.record.post.uri,
            },
          },
        };
        return await bluesky.pdsAgent.post(record);
      },
      {
        pending: (state) => {
          console.log("chatPost pending");
        },
        fulfilled: (state, action) => {
          console.log("chatPost fulfilled", action.payload);
        },
        rejected: (state, action) => {
          console.error("chatPost rejected", action.error);
          // state.status = "failed";
        },
      },
    ),

    createStreamKeyRecord: create.asyncThunk(
      async ({ store }: { store: boolean }, thunkAPI) => {
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
        if (!did) {
          throw new Error("No DID");
        }
        const keypair = await Secp256k1Keypair.create({ exportable: true });
        const exportedKey = await keypair.export();
        const didBytes = new TextEncoder().encode(did);
        const combinedKey = new Uint8Array([...exportedKey, ...didBytes]);
        const multibaseKey = bytesToMultibase(combinedKey, "base58btc");
        const hexKey = Array.from(exportedKey)
          .map((b) => b.toString(16).padStart(2, "0"))
          .join("");
        const account = await privateKeyToAccount(`0x${hexKey}`);
        const newKey = {
          privateKey: multibaseKey,
          did: keypair.did(),
          address: account.address.toLowerCase(),
        };
        const record: PlaceStreamKey.Record = {
          signingKey: keypair.did(),
          createdAt: new Date().toISOString(),
        };
        await bluesky.pdsAgent.com.atproto.repo.createRecord({
          repo: did,
          collection: "place.stream.key",
          record,
        });
        if (store) {
          await Storage.setItem(STORED_KEY_KEY, JSON.stringify(newKey));
        }
        return newKey;
      },
      {
        pending: (state) => {
          console.log("golivePost pending");
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            newKey: action.payload,
            storedKey: action.meta.arg.store ? action.payload : null,
          };
        },
        rejected: (state, action) => {
          console.error("getProfile rejected", action.error);
          // state.status = "failed";
        },
      },
    ),

    clearStreamKeyRecord: create.reducer((state) => {
      return {
        ...state,
        newKey: null,
      };
    }),

    setPDS: create.asyncThunk(
      async (pds: string, thunkAPI) => {
        await Storage.setItem("pdsURL", pds);
        return pds;
      },
      {
        pending: (state, action) => {
          return {
            ...state,
            pds: {
              ...state.pds,
              loading: true,
            },
          };
        },
        fulfilled: (state, action) => {
          // document.location.href = action.payload.toString();
          console.log("setPDS fulfilled", action.payload);
          return {
            ...state,
            pds: {
              ...state.pds,
              loading: false,
              url: action.payload,
            },
          };
        },
        rejected: (state, action) => {
          return {
            ...state,
            pds: {
              ...state.pds,
              loading: false,
              error: action.error?.message ?? null,
            },
          };
        },
      },
    ),

    createLivestreamRecord: create.asyncThunk(
      async ({ title }: { title }, thunkAPI) => {
        const now = new Date();
        const { bluesky, streamplace } = thunkAPI.getState() as {
          bluesky: BlueskyState;
          streamplace: StreamplaceState;
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
        if (!did) {
          throw new Error("No DID");
        }
        const newPost = (await thunkAPI.dispatch(
          golivePost({ text: title, now }),
        )) as { payload: { uri: string; cid: string } };
        const record: PlaceStreamLivestream.Record = {
          title: title,
          url: streamplace.url,
          createdAt: new Date().toISOString(),
          post: {
            uri: newPost.payload.uri,
            cid: newPost.payload.cid,
          },
        };
        await bluesky.pdsAgent.com.atproto.repo.createRecord({
          repo: did,
          collection: "place.stream.livestream",
          record,
        });
        return record;
      },
      {
        pending: (state) => {
          return {
            ...state,
            newLivestream: {
              loading: true,
              error: null,
              record: null,
            },
          };
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            newLivestream: {
              loading: false,
              error: null,
              record: action.payload,
            },
          };
        },
        rejected: (state, action) => {
          console.error("getProfile rejected", action.error);
          return {
            ...state,
            newLivestream: {
              loading: false,
              error: action.error?.message ?? null,
              record: null,
            },
          };
        },
      },
    ),
  }),

  // You can define your selectors here. These selectors receive the slice
  // state as their first argument.
  selectors: {
    selectOAuthSession: (bluesky) => bluesky.oauthSession,
    selectPDS: (bluesky) => bluesky.pds,
    selectLogin: (bluesky) => bluesky.login,
    selectProfiles: (bluesky) => bluesky.profiles,
    selectStoredKey: (bluesky) => bluesky.storedKey,
    selectUserProfile: (bluesky) => {
      const did = bluesky.oauthSession?.did;
      if (!did) return null;
      return bluesky.profiles[did];
    },
    selectIsReady: (bluesky) => {
      if (bluesky.status === "start") {
        return false;
      } else if (bluesky.status === "loggedOut") {
        return true;
      }
      if (!bluesky.oauthSession) {
        return false;
      }
      const profile = blueskySlice.selectors.selectUserProfile({ bluesky });
      if (!profile) {
        return false;
      }

      return true;
    },
    selectNewLivestream: (bluesky) => bluesky.newLivestream,
  },
});

// Action creators are generated for each case reducer function.
export const {
  loadOAuthClient,
  login,
  getProfile,
  logout,
  golivePost,
  oauthCallback,
  setPDS,
  createStreamKeyRecord,
  clearStreamKeyRecord,
  createLivestreamRecord,
  chatPost,
} = blueskySlice.actions;

// Selectors returned by `slice.selectors` take the root state as their first argument.
export const {
  selectOAuthSession,
  selectProfiles,
  selectUserProfile,
  selectPDS,
  selectLogin,
  selectStoredKey,
  selectIsReady,
  selectNewLivestream,
} = blueskySlice.selectors;
