import {
  Agent,
  AppBskyActorGetProfiles,
  AppBskyFeedPost,
  AppBskyGraphBlock,
  BlobRef,
  RichText,
} from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { bytesToMultibase, Secp256k1Keypair } from "@atproto/crypto";
import { OAuthSession } from "@atproto/oauth-client";
import { DID_KEY, hydrate, STORED_KEY_KEY } from "features/base/baseSlice";
import { openLoginLink } from "features/platform/platformSlice";
import { PlayersState } from "features/player/playerSlice";
import {
  setURL,
  StreamplaceState,
} from "features/streamplace/streamplaceSlice";
import Storage from "storage";
import {
  LivestreamViewHydrated,
  PlaceStreamChatProfile,
  PlaceStreamKey,
  PlaceStreamLivestream,
  StreamplaceAgent,
} from "streamplace";
import { isWeb } from "tamagui";
import { privateKeyToAccount } from "viem/accounts";
import { createAppSlice } from "../../hooks/createSlice";
import { BlueskyState } from "./blueskyTypes";
import createOAuthClient from "./oauthClient";

const initialState: BlueskyState = {
  status: "start",
  oauthState: null,
  oauthSession: null,
  pdsAgent: null,
  anonPDSAgent: null,
  profiles: {},
  profileCache: {},
  client: null,
  login: {
    loading: false,
    error: null,
  },
  chatProfile: {
    loading: false,
    error: null,
    profile: null,
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
  pdsAgent: StreamplaceAgent,
  profile: ProfileViewDetailed,
  customThumbnail?: Blob,
) => {
  if (customThumbnail) {
    let tries = 0;
    try {
      let thumbnail = await pdsAgent.uploadBlob(customThumbnail);

      while (
        thumbnail.data.blob.size === 0 &&
        customThumbnail.size !== 0 &&
        tries < 3
      ) {
        console.warn(
          "Reuploading blob as blob sizes don't match! Blob size recieved is",
          thumbnail.data.blob.size,
          "and sent blob size is",
          customThumbnail.size,
        );
        thumbnail = await pdsAgent.uploadBlob(customThumbnail);
      }

      if (tries === 3) {
        throw new Error("Could not successfully upload blob (tried thrice)");
      }

      if (thumbnail.success) {
        console.log("Successfully uploaded thumbnail");
        return thumbnail.data.blob;
      }
    } catch (e) {
      throw new Error("Error uploading thumbnail: " + e);
    }
  }
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
    builder.addCase(setURL, (state, action) => {
      return {
        ...state,
        anonPDSAgent: new StreamplaceAgent(action.payload) as any,
      };
    });
    builder.addDefaultCase((state, action) => {
      const err = (action as any).error as { message: string };
      if (
        typeof err === "object" &&
        typeof err?.message === "string" &&
        err.message.includes("oauth session revoked")
      ) {
        Storage.removeItem("did").catch((e) => {
          console.error("Error removing did", e);
        });
        Storage.removeItem(STORED_KEY_KEY).catch((e) => {
          console.error("Error removing stored key", e);
        });
        const u = new URL(document.location.href);
        return {
          ...state,
          oauthSession: null,
          status: "loggedOut",
          pdsAgent: null,
        };
      }
      return state;
    });
  },
  reducers: (create) => ({
    loadOAuthClient: create.asyncThunk(
      async (_, { getState }) => {
        const { streamplace } = getState() as { streamplace: StreamplaceState };
        const client = await createOAuthClient(streamplace.url);
        const anonPDSAgent = new StreamplaceAgent(streamplace.url);
        const maybeDIDs = await Promise.all([
          Storage.getItem(DID_KEY),
          Storage.getItem("@@atproto/oauth-client-browser(sub)"),
          Storage.getItem("@@atproto/oauth-client-react-native:did:(sub)"),
        ]);
        const did = maybeDIDs.find((d) => d !== null) || null;
        let session: OAuthSession | null = null;
        if (did) {
          try {
            session = await client.restore(did);
          } catch (e) {
            console.error("Error restoring session", e);
          }
        }
        // let initResult = await client.init();
        return { client, session, anonPDSAgent };
      },
      {
        pending: (state) => {
          return {
            ...state,
            status: "start",
          };
        },
        fulfilled: (state, action) => {
          const { client, session, anonPDSAgent } = action.payload;
          console.log("loadOAuthClient fulfilled", action.payload);
          if (session) {
            Storage.setItem(DID_KEY, session.did).catch((e) => {
              console.error("Error setting did", e);
            });
            return {
              ...state,
              client: client as any,
              status: "loggedIn",
              oauthSession: session as any,
              pdsAgent: new StreamplaceAgent(session) as any, // idk why this is needed
              anonPDSAgent: anonPDSAgent as any,
            } as any;
          }
          return {
            ...state,
            status: "loggedOut",
            client: client,
            anonPDSAgent: anonPDSAgent,
          };
        },
        rejected: (state, { error }) => {
          return {
            ...state,
            // status: "loggedOut",
          };
        },
      },
    ),

    oauthError: create.reducer(
      (
        state,
        { payload }: { payload: { error: string; description: string } },
      ) => {
        return {
          ...state,
          login: {
            loading: false,
            error: payload.description || payload.error,
          },
          status: "loggedOut",
        };
      },
    ),

    login: create.asyncThunk(
      async (handle: string, thunkAPI) => {
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
        const u = await bluesky.client.authorize(handle, {});
        thunkAPI.dispatch(openLoginLink(u.toString()));
        // cheeky 500ms delay so you don't see the text flash back
        await new Promise((resolve) => setTimeout(resolve, 5000));
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
          console.error("login rejected", action.error);
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
            status: "loggedOut",
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
          return {
            ...state,
            status: "loggedOut",
          };
        },
      },
    ),

    getProfiles: create.asyncThunk(
      async (actors: string[], thunkAPI) => {
        if (actors.length > 25) {
          throw Error("Requested too many actors! (max 25 actors)");
        }
        const { bluesky } = thunkAPI.getState() as {
          bluesky: BlueskyState;
        };
        // unauthed request to Bluesky Appview
        const bskyAgent = new Agent("https://public.api.bsky.app");

        return await bskyAgent.getProfiles({
          actors: actors,
        });
      },
      {
        pending: (state) => {
          // state.status = "loading";
        },
        fulfilled: (state, action) => {
          let payload: AppBskyActorGetProfiles.Response = action.payload;
          let parsedProfiles = {};
          console.log(payload);
          payload.data.profiles.forEach((p) => {
            parsedProfiles[p.did] = p;
          });

          return {
            ...state,
            profileCache: {
              ...state.profileCache,
              ...parsedProfiles,
            },
          };
        },
        rejected: (state, action) => {
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
          if (params.has("error")) {
            thunkAPI.dispatch(
              oauthError({
                error: params.get("error") ?? "",
                description: params.get("error_description") ?? "",
              }),
            );
          }
          throw new Error("Missing params, got: " + url);
        }
        const { streamplace } = thunkAPI.getState() as {
          streamplace: StreamplaceState;
        };
        const client = await createOAuthClient(streamplace.url);
        try {
          const ret = await client.callback(params);
          await Storage.setItem(DID_KEY, ret.session.did);
          return { session: ret.session as any, client };
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
          return {
            ...state,
            status: "start",
          };
        },
        fulfilled: (state, action) => {
          console.log("oauthCallback fulfilled", action.payload);
          return {
            ...state,
            client: action.payload.client as any,
            oauthSession: action.payload.session as any,
            pdsAgent: new StreamplaceAgent(action.payload.session) as any,
            status: "loggedIn",
          };
        },
        rejected: (state, action) => {
          console.error("oauthCallback rejected", action.error);
          return {
            ...state,
            status: "loggedOut",
          };
        },
      },
    ),

    golivePost: create.asyncThunk(
      async (
        {
          text,
          now,
          thumbnail,
        }: { text: string; now: Date; thumbnail?: BlobRef },
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

        const linkUrl = `${u.protocol}//${u.host}/${profile.handle}?${params.toString()}`;
        const prefix = `🔴 LIVE `;
        const textUrl = `${u.protocol}//${u.host}/${profile.handle}`;
        const suffix = ` ${text}`;
        const content = prefix + textUrl + suffix;

        const rt = new RichText({ text: content });
        rt.detectFacetsWithoutResolution();

        const record: AppBskyFeedPost.Record = {
          $type: "app.bsky.feed.post",
          text: content,
          "place.stream.livestream": {
            url: linkUrl,
            title: text,
          },
          facets: rt.facets,
          createdAt: now.toISOString(),
        };
        record.embed = {
          $type: "app.bsky.embed.external",
          external: {
            description: text,
            thumb: thumbnail,
            title: `@${profile.handle} is 🔴LIVE on ${u.host}!`,
            uri: linkUrl,
          },
        };
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

    createBlockRecord: create.asyncThunk(
      async ({ subjectDID }: { subjectDID: string }, thunkAPI) => {
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
        const record: AppBskyGraphBlock.Record = {
          $type: "app.bsky.graph.block",
          subject: subjectDID,
          createdAt: new Date().toISOString(),
        };
        return await bluesky.pdsAgent.com.atproto.repo.createRecord({
          repo: did,
          collection: "app.bsky.graph.block",
          record,
        });
      },
      {
        pending: (state) => {
          console.log("createBlockRecord pending");
        },
        fulfilled: (state, action) => {
          console.log("createBlockRecord fulfilled", action.payload);
        },
        rejected: (state, action) => {
          console.error("createBlockRecord rejected", action.error);
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
          console.error("createStreamKeyRecord rejected", action.error);
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
      async (
        { title, customThumbnail }: { title: string; customThumbnail?: Blob },
        thunkAPI,
      ) => {
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

        let thumbnail: BlobRef | undefined = undefined;

        const u = new URL(streamplace.url);

        if (customThumbnail) {
          try {
            thumbnail = await uploadThumbnail(
              profile.handle,
              u,
              bluesky.pdsAgent,
              profile,
              customThumbnail,
            );
          } catch (e) {
            throw new Error(`Custom thumbnail upload failed ${e}`);
          }
        } else {
          // No custom thumbnail: fetch the server-side image and upload it
          try {
            const thumbnailRes = await fetch(
              `${u.protocol}//${u.host}/api/playback/${profile.handle}/stream.png`,
            );
            if (!thumbnailRes.ok) {
              throw new Error(
                `Failed to fetch thumbnail: ${thumbnailRes.status})`,
              );
            }
            const thumbnailBlob = await thumbnailRes.blob();
            console.log(thumbnailBlob);
            thumbnail = await uploadThumbnail(
              profile.handle,
              u,
              bluesky.pdsAgent,
              profile,
              thumbnailBlob,
            );
          } catch (e) {
            throw new Error(`Thumbnail upload failed ${e}`);
          }
        }

        const newPostAction = await thunkAPI.dispatch(
          golivePost({ text: title, now, thumbnail }),
        );

        if (!newPostAction || newPostAction.type.endsWith("/rejected")) {
          throw new Error(
            `Failed to create post: ${(newPostAction as any)?.error?.message || "Unknown error"}`,
          );
        }

        const newPost = newPostAction as {
          payload: { uri: string; cid: string };
        };

        if (!newPost.payload?.uri || !newPost.payload?.cid) {
          throw new Error(
            "Cannot read properties of undefined (reading 'uri' or 'cid')",
          );
        }

        const record: PlaceStreamLivestream.Record = {
          title: title,
          url: streamplace.url,
          createdAt: new Date().toISOString(),
          post: {
            uri: newPost.payload.uri,
            cid: newPost.payload.cid,
          },
          thumb: thumbnail,
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
          console.error("createLivestreamRecord rejected", action.error);
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

    updateLivestreamRecord: create.asyncThunk(
      async (
        {
          title,
          livestream,
        }: { title: string; livestream: LivestreamViewHydrated | null },
        thunkAPI,
      ) => {
        const now = new Date();
        const { bluesky, streamplace, player } = thunkAPI.getState() as {
          bluesky: BlueskyState;
          streamplace: StreamplaceState;
          player: PlayersState;
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

        let oldRecord = livestream;
        if (!oldRecord) {
          throw new Error("No latest record");
        }

        let rkey = oldRecord.uri.split("/").pop();
        let oldRecordValue: PlaceStreamLivestream.Record = oldRecord.record;

        if (!rkey) {
          throw new Error("No rkey?");
        }

        console.log("Updating rkey", rkey);

        const record: PlaceStreamLivestream.Record = {
          title: title,
          url: streamplace.url,
          createdAt: new Date().toISOString(),
          post: oldRecordValue.post,
        };

        await bluesky.pdsAgent.com.atproto.repo.putRecord({
          repo: did,
          collection: "place.stream.livestream",
          rkey,
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
          console.error("createLivestreamRecord rejected", action.error);
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

    getChatProfileRecordFromPDS: create.asyncThunk(
      async (_, thunkAPI) => {
        const { bluesky } = thunkAPI.getState() as { bluesky: BlueskyState };
        const did = bluesky.oauthSession?.did;
        if (!did) {
          throw new Error("No DID");
        }
        const profile = bluesky.profiles[did];
        if (!profile) {
          throw new Error("No profile");
        }
        if (!bluesky.pdsAgent) {
          throw new Error("No agent");
        }
        const res = await bluesky.pdsAgent.com.atproto.repo.getRecord({
          repo: did,
          collection: "place.stream.chat.profile",
          rkey: "self",
        });
        if (!res.success) {
          throw new Error("Failed to get chat profile record");
        }

        if (PlaceStreamChatProfile.isRecord(res.data.value)) {
          return res.data.value;
        } else {
          console.log("not a record", res.data.value);
        }
        return null;
      },
      {
        pending: (state) => {
          return {
            ...state,
            chatProfile: {
              loading: true,
              error: null,
              profile: null,
            },
          };
        },
        fulfilled: (state, action) => {
          if (!action.payload) {
            return state;
          }
          return {
            ...state,
            chatProfile: {
              loading: false,
              error: null,
              profile: action.payload,
            },
          };
        },
      },
    ),

    createChatProfileRecord: create.asyncThunk(
      async (
        { red, green, blue }: { red: number; green: number; blue: number },
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
        if (!did) {
          throw new Error("No DID");
        }

        const chatProfile: PlaceStreamChatProfile.Record = {
          color: {
            red: red,
            green: green,
            blue: blue,
          },
        };

        const res = await bluesky.pdsAgent.com.atproto.repo.putRecord({
          repo: did,
          collection: "place.stream.chat.profile",
          record: chatProfile,
          rkey: "self",
        });
        if (!res.success) {
          throw new Error("Failed to create chat profile record");
        }
        return chatProfile;
      },
      {
        pending: (state) => {
          return {
            ...state,
            chatProfile: {
              loading: true,
              error: null,
              profile: null,
            },
          };
        },
        fulfilled: (state, action) => {
          return {
            ...state,
            chatProfile: {
              loading: false,
              error: null,
              profile: action.payload,
            },
          };
        },
        rejected: (state, action) => {
          console.error("createChatProfileRecord rejected", action.error);
          return {
            ...state,
            chatProfile: {
              loading: false,
              error: action.error?.message ?? null,
              profile: null,
            },
          };
        },
      },
    ),

    followUser: create.asyncThunk(
      async (subjectDID: string, thunkAPI) => {
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
        await bluesky.pdsAgent.follow(subjectDID);

        return { subjectDID };
      },
      {
        pending: (state) => {
          console.log("followUser pending");
        },
        fulfilled: (state, action) => {
          console.log("followUser fulfilled", action.payload);
        },
        rejected: (state, action) => {
          console.error("followUser rejected", action.error);
        },
      },
    ),

    unfollowUser: create.asyncThunk(
      async (
        { subjectDID, followUri }: { subjectDID: string; followUri?: string },
        thunkAPI,
      ) => {
        const { bluesky, streamplace } = thunkAPI.getState() as {
          bluesky: BlueskyState;
          streamplace: StreamplaceState;
        };
        let agent;
        if (!bluesky.pdsAgent) {
          throw new Error("No agent");
        }
        const did = bluesky.oauthSession?.did;
        if (!did) {
          throw new Error("No DID");
        }

        if (followUri) {
          await bluesky.pdsAgent.deleteFollow(followUri);
        } else {
          const res = await fetch(
            `${streamplace.url}/xrpc/place.stream.graph.getFollowingUser?subjectDID=${encodeURIComponent(subjectDID)}&userDID=${encodeURIComponent(did)}`,
            {
              credentials: "include",
            },
          );
          const data = await res.json();

          if (!data.follow || !data.follow.uri) {
            throw new Error("Follow record not found");
          }

          await bluesky.pdsAgent.deleteFollow(data.follow.uri);
        }

        return { subjectDID };
      },
      {
        pending: (state) => {
          console.log("unfollowUser pending");
        },
        fulfilled: (state, action) => {
          console.log("unfollowUser fulfilled", action.payload);
        },
        rejected: (state, action) => {
          console.error("unfollowUser rejected", action.error);
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
    selectChatProfile: (bluesky) => bluesky.chatProfile,
    selectCachedProfiles: (bluesky) => bluesky.profileCache,
  },
});

// Action creators are generated for each case reducer function.
export const {
  loadOAuthClient,
  login,
  getProfile,
  getProfiles,
  logout,
  golivePost,
  oauthCallback,
  setPDS,
  oauthError,
  createStreamKeyRecord,
  clearStreamKeyRecord,
  createLivestreamRecord,
  updateLivestreamRecord,
  createChatProfileRecord,
  getChatProfileRecordFromPDS,
  createBlockRecord,
  followUser,
  unfollowUser,
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
  selectChatProfile,
  selectCachedProfiles,
} = blueskySlice.selectors;
