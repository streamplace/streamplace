import {
  Agent,
  AppBskyFeedPost,
  AppBskyGraphBlock,
  BlobRef,
  RichText,
} from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { OutputSchema } from "@atproto/api/dist/client/types/com/atproto/repo/listRecords";
import { bytesToMultibase, Secp256k1Keypair } from "@atproto/crypto";
import { OAuthSession } from "@atproto/oauth-client";
import { storage } from "@streamplace/components";
import { Platform } from "react-native";
import { AppStore } from "store";
import {
  PlaceStreamChatProfile,
  PlaceStreamKey,
  PlaceStreamLivestream,
  PlaceStreamServerSettings,
  StreamplaceAgent,
} from "streamplace";
import { privateKeyToAccount } from "viem/accounts";
import { StateCreator } from "zustand";
import createOAuthClient, {
  StreamplaceOAuthClient,
} from "../../features/bluesky/oauthClient";
import { DID_KEY, STORED_KEY_KEY, StreamKey } from "./baseSlice";

type NewLivestream = {
  loading: boolean;
  error: string | null;
  record: PlaceStreamLivestream.Record | null;
};

export interface BlueskySlice {
  authStatus: "start" | "loggedIn" | "loggedOut";
  oauthState: null | string;
  oauthSession?: null | OAuthSession;
  pdsAgent: null | StreamplaceAgent;
  anonPDSAgent: null | StreamplaceAgent;
  profiles: { [key: string]: ProfileViewDetailed };
  profileCache: { [key: string]: ProfileViewDetailed };
  client: null | StreamplaceOAuthClient;
  loginState: {
    loading: boolean;
    error: null | string;
  };
  pds: {
    url: string;
    loading: boolean;
    error: null | string;
  };
  newKey: null | StreamKey;
  storedKey: null | StreamKey;
  isDeletingKey: boolean;
  streamKeysResponse: {
    loading: boolean;
    error: null | string;
    records: null | OutputSchema;
  };
  newLivestream: null | NewLivestream;
  chatProfile: {
    loading: boolean;
    error: null | string;
    profile: null | PlaceStreamChatProfile.Record;
  };
  serverSettings: null | PlaceStreamServerSettings.Record;
  returnRoute: null | { name: string; params?: any };
  notification: {
    message: string;
    type: "error" | "success" | "info";
  } | null;
  // actions
  clearNotification: () => void;
  loadOAuthClient: () => Promise<void>;
  oauthError: (error: string, description: string) => void;
  login: (
    handle: string,
    openLoginLink: (url: string) => Promise<void>,
  ) => Promise<void>;
  logout: () => Promise<void>;
  getProfile: (actor: string) => Promise<void>;
  getProfiles: (actors: string[]) => Promise<void>;
  oauthCallback: (url: string) => Promise<void>;
  setReturnRoute: (route: { name: string; params?: any } | null) => void;
  showLoginModal: boolean;
  openLoginModal: (returnRoute?: { name: string; params?: any }) => void;
  closeLoginModal: () => void;
  golivePost: (
    text: string,
    now: Date,
    thumbnail?: BlobRef,
  ) => Promise<{ uri: string; cid: string }>;
  createBlockRecord: (subjectDID: string) => Promise<void>;
  createStreamKeyRecord: (store: boolean) => Promise<void>;
  clearStreamKeyRecord: () => void;
  getStreamKeyRecords: () => Promise<void>;
  deleteStreamKeyRecord: (rkey: string) => Promise<void>;
  setPDS: (pds: string) => Promise<void>;
  createLivestreamRecord: (
    title: string,
    customThumbnail?: Blob,
  ) => Promise<void>;
  updateLivestreamRecord: (title: string, livestream: any) => Promise<void>;
  getChatProfileRecordFromPDS: () => Promise<void>;
  createChatProfileRecord: (
    red: number,
    green: number,
    blue: number,
  ) => Promise<void>;
  followUser: (subjectDID: string) => Promise<void>;
  unfollowUser: (subjectDID: string, followUri?: string) => Promise<void>;
  getServerSettingsFromPDS: () => Promise<void>;
  createServerSettingsRecord: (debugRecording: boolean) => Promise<void>;
}

const clearQueryParams = () => {
  if (Platform.OS !== "web") {
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

export const createBlueskySlice: StateCreator<
  AppStore,
  [],
  [],
  BlueskySlice
> = (set, get) => ({
  authStatus: "start",
  oauthState: null,
  oauthSession: undefined,
  pdsAgent: null,
  anonPDSAgent: null,
  profiles: {},
  profileCache: {},
  client: null,
  loginState: {
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
  isDeletingKey: false,
  streamKeysResponse: {
    loading: true,
    error: null,
    records: null,
  },
  newLivestream: null,
  chatProfile: {
    loading: false,
    error: null,
    profile: null,
  },
  serverSettings: null,
  returnRoute: null,
  showLoginModal: false,
  notification: null,

  clearNotification: () => {
    set({ notification: null });
  },

  setReturnRoute: async (route: { name: string; params?: any } | null) => {
    console.log("setReturnRoute:", route);
    if (route) {
      await storage.setItem("returnRoute", JSON.stringify(route));
    } else {
      await storage.removeItem("returnRoute");
    }
    set({ returnRoute: route });
  },

  openLoginModal: async (returnRoute?: { name: string; params?: any }) => {
    console.log("openLoginModal with returnRoute:", returnRoute);
    if (returnRoute) {
      await storage.setItem("returnRoute", JSON.stringify(returnRoute));
    }
    set({ showLoginModal: true, returnRoute: returnRoute || null });
  },

  closeLoginModal: () => {
    console.log("closeLoginModal");
    set({ showLoginModal: false });
  },

  loadOAuthClient: async () => {
    set({ authStatus: "start" });
    try {
      const streamplaceUrl = get().url;
      const client = await createOAuthClient(streamplaceUrl);
      const anonPDSAgent = new StreamplaceAgent(streamplaceUrl);
      const maybeDIDs = await Promise.all([
        storage.getItem(DID_KEY),
        storage.getItem("@@atproto/oauth-client-browser(sub)"),
        storage.getItem("@@atproto/oauth-client-react-native:did:(sub)"),
      ]);
      const did = maybeDIDs.find((d) => d !== null) || null;
      let session: OAuthSession | null = null;
      if (did) {
        try {
          session = await client.restore(did);
        } catch (e) {
          console.error("Error restoring session", e);
          // oh well, delete the session and start fresh
          await storage.removeItem(DID_KEY);
          await storage.removeItem("@@atproto/oauth-client-browser(sub)");
          await storage.removeItem(
            "@@atproto/oauth-client-react-native:did:(sub)",
          );
        }
      }
      console.log("loadOAuthClient fulfilled", {
        client,
        session,
        anonPDSAgent,
      });
      console.log("session?", session);
      if (session) {
        storage.setItem(DID_KEY, session.did).catch((e) => {
          console.error("Error setting did", e);
        });
        set({
          client,
          authStatus: "loggedIn",
          oauthSession: session,
          pdsAgent: new StreamplaceAgent(session),
          anonPDSAgent,
        });
      } else {
        set({
          oauthSession: session,
          authStatus: "loggedOut",
          client,
          anonPDSAgent,
        });
      }
    } catch (error) {
      console.error("loadOAuthClient error", error);
    }
  },

  oauthError: (error: string, description: string) => {
    const message = description || error || "authentication failed";
    set({
      loginState: {
        loading: false,
        error: message,
      },
      authStatus: "loggedOut",
      notification: {
        message,
        type: "error",
      },
    });
  },

  login: async (
    handle: string,
    openLoginLink: (url: string) => Promise<void>,
  ) => {
    console.log("Logging in");
    set({
      loginState: {
        loading: true,
        error: null,
      },
    });
    try {
      const state = get() as BlueskySlice;
      await state.loadOAuthClient();
      const updatedState = get() as BlueskySlice;
      if (!updatedState.client) {
        throw new Error("No client");
      }
      console.log("Authorizing");
      const u = await updatedState.client.authorize(handle, {});
      if (
        typeof document !== "undefined" &&
        document.location.href.startsWith("http://127.0.0.1")
      ) {
        const hostUrl = new URL(document.location.href);
        u.host = hostUrl.host;
        u.protocol = hostUrl.protocol;
      }
      console.log("Opening link");
      await openLoginLink(u.toString());
      // cheeky 500ms delay so you don't see the text flash back
      await new Promise((resolve) => setTimeout(resolve, 5000));
      set({
        loginState: {
          loading: false,
          error: null,
        },
      });
    } catch (error) {
      console.error("login rejected", error);
      set({
        loginState: {
          loading: false,
          error: error?.message ?? null,
        },
        notification: {
          message: error?.message || "unknown error",
          type: "error",
        },
      });
    }
  },

  logout: async () => {
    await storage.removeItem("did");
    await storage.removeItem(STORED_KEY_KEY);
    const state = get() as BlueskySlice;
    if (!state.oauthSession) {
      throw new Error("No oauth session");
    }
    await state.oauthSession.signOut();
    set({
      oauthSession: null,
      pdsAgent: null,
      authStatus: "loggedOut",
    });
  },

  getProfile: async (actor: string) => {
    try {
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const result = await state.pdsAgent.getProfile({ actor });
      clearQueryParams();
      set((s) => ({
        authStatus: "loggedIn",
        profiles: {
          ...(s as BlueskySlice).profiles,
          [actor]: result.data,
        },
      }));
    } catch (error) {
      clearQueryParams();
      set({ authStatus: "loggedOut" });
    }
  },

  getProfiles: async (actors: string[]) => {
    if (actors.length > 25) {
      throw Error("Requested too many actors! (max 25 actors)");
    }
    try {
      const bskyAgent = new Agent("https://public.api.bsky.app");
      const payload = await bskyAgent.getProfiles({ actors });
      let parsedProfiles = {};
      console.log(payload);
      payload.data.profiles.forEach((p) => {
        parsedProfiles[p.did] = p;
      });
      set((s) => ({
        profileCache: {
          ...(s as BlueskySlice).profileCache,
          ...parsedProfiles,
        },
      }));
    } catch (error) {
      console.error("getProfiles error", error);
    }
  },

  oauthCallback: async (url: string) => {
    set({ authStatus: "start" });
    try {
      console.log("oauthCallback", url);
      if (!url.includes("?")) {
        throw new Error("No query params");
      }
      const params = new URLSearchParams(url.split("?")[1]);
      if (!(params.has("code") && params.has("state") && params.has("iss"))) {
        if (params.has("error")) {
          const blueskySlice = get() as BlueskySlice;
          blueskySlice.oauthError(
            params.get("error") ?? "",
            params.get("error_description") ?? "",
          );
        }
        throw new Error("Missing params, got: " + url);
      }
      const streamplaceUrl = get().url;
      const client = await createOAuthClient(streamplaceUrl);
      try {
        const ret = await client.callback(params);
        await storage.setItem(DID_KEY, ret.session.did);
        console.log("oauthCallback fulfilled", {
          session: ret.session,
          client,
        });
        set({
          client,
          oauthSession: ret.session,
          pdsAgent: new StreamplaceAgent(ret.session),
          authStatus: "loggedIn",
        });
      } catch (e) {
        let message = e.message;
        while (e.cause) {
          message = `${message}: ${e.cause.message}`;
          e = e.cause;
        }
        console.error("oauthCallback error", message);
        set({
          authStatus: "loggedOut",
          notification: {
            message,
            type: "error",
          },
        });
        throw e;
      }
    } catch (error) {
      console.error("oauthCallback rejected", error);
      const message = error?.message || "authentication failed";
      set({
        authStatus: "loggedOut",
        notification: {
          message,
          type: "error",
        },
      });
    }
  },

  golivePost: async (text: string, now: Date, thumbnail?: BlobRef) => {
    const state = get() as BlueskySlice;
    if (!state.pdsAgent) {
      throw new Error("No agent");
    }
    const did = state.oauthSession?.did;
    if (!did) {
      throw new Error("No DID");
    }
    const profile = state.profiles[did];
    if (!profile) {
      throw new Error("No profile");
    }
    const streamplaceUrl = get().url;
    const u = new URL(streamplaceUrl);
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
      langs: ["en"],
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
    return await state.pdsAgent.post(record);
  },

  createBlockRecord: async (subjectDID: string) => {
    try {
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }
      const record: AppBskyGraphBlock.Record = {
        $type: "app.bsky.graph.block",
        subject: subjectDID,
        createdAt: new Date().toISOString(),
      };
      await state.pdsAgent.com.atproto.repo.createRecord({
        repo: did,
        collection: "app.bsky.graph.block",
        record,
      });
      console.log("createBlockRecord fulfilled");
    } catch (error) {
      console.error("createBlockRecord rejected", error);
    }
  },

  createStreamKeyRecord: async (store: boolean) => {
    try {
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
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

      let platform: string = Platform.OS;

      if (Platform.OS === "web" && window && window.navigator) {
        let splitUA = window.navigator.userAgent
          .split(" ")
          .pop()
          ?.split("/")[0];
        if (splitUA) {
          platform = splitUA;
        }
      } else if (platform === "android") {
        platform = "Android";
      } else if (platform === "ios") {
        platform = "iOS";
      } else if (platform === "macos") {
        platform = "macOS";
      } else if (platform === "windows") {
        platform = "Windows";
      }

      const record: PlaceStreamKey.Record = {
        $type: "place.stream.key",
        signingKey: keypair.did(),
        createdAt: new Date().toISOString(),
        createdBy: "Streamplace on " + platform,
      };
      await state.pdsAgent.com.atproto.repo.createRecord({
        repo: did,
        collection: "place.stream.key",
        record,
      });
      if (store) {
        await storage.setItem(STORED_KEY_KEY, JSON.stringify(newKey));
      }
      set({
        newKey: newKey,
        storedKey: store ? newKey : null,
      });
    } catch (error) {
      console.error("createStreamKeyRecord rejected", error);
    }
  },

  clearStreamKeyRecord: () => {
    set({ newKey: null });
  },

  getStreamKeyRecords: async () => {
    set({
      streamKeysResponse: {
        loading: true,
        error: null,
        records: null,
      },
    });
    try {
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }
      const result = await state.pdsAgent.com.atproto.repo.listRecords({
        repo: did,
        collection: "place.stream.key",
        limit: 100,
      });
      console.log(result);
      set({
        streamKeysResponse: {
          loading: false,
          error: null,
          records: result.data,
        },
      });
    } catch (error) {
      console.error("listStreamKeyRecords rejected", error);
      set({
        streamKeysResponse: {
          loading: false,
          error: error?.message ?? null,
          records: null,
        },
      });
    }
  },

  deleteStreamKeyRecord: async (rkey: string) => {
    set({ isDeletingKey: true });
    try {
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }
      await state.pdsAgent.com.atproto.repo.deleteRecord({
        repo: did,
        collection: "place.stream.key",
        rkey,
      });
      let records = state.streamKeysResponse.records
        ? state.streamKeysResponse.records.records.filter(
            (r) => r.uri.split("/").pop() !== rkey,
          )
        : [];
      set({
        isDeletingKey: false,
        streamKeysResponse: {
          ...state.streamKeysResponse,
          records: {
            ...state.streamKeysResponse.records!,
            records,
          },
        },
      });
    } catch (error) {
      console.error("deleteStreamKeyRecord rejected", error);
      set({ isDeletingKey: false });
    }
  },

  setPDS: async (pds: string) => {
    set({
      pds: {
        ...(get() as BlueskySlice).pds,
        loading: true,
      },
    });
    try {
      await storage.setItem("pdsURL", pds);
      console.log("setPDS fulfilled", pds);
      set({
        pds: {
          ...(get() as BlueskySlice).pds,
          loading: false,
          url: pds,
        },
      });
    } catch (error) {
      set({
        pds: {
          ...(get() as BlueskySlice).pds,
          loading: false,
          error: error?.message ?? null,
        },
      });
    }
  },

  createLivestreamRecord: async (title: string, customThumbnail?: Blob) => {
    set({
      newLivestream: {
        loading: true,
        error: null,
        record: null,
      },
    });
    try {
      const now = new Date();
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }

      let thumbnail: BlobRef | undefined = undefined;
      const streamplaceUrl = get().url;
      const u = new URL(streamplaceUrl);

      if (customThumbnail) {
        try {
          thumbnail = await uploadThumbnail(
            profile.handle,
            u,
            state.pdsAgent,
            profile,
            customThumbnail,
          );
        } catch (e) {
          throw new Error(`Custom thumbnail upload failed ${e}`);
        }
      } else {
        let tries = 0;
        try {
          for (; tries < 3; tries++) {
            try {
              console.log(
                `Fetching thumbnail from ${u.protocol}//${u.host}/api/playback/${profile.handle}/stream.png`,
              );
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
                state.pdsAgent,
                profile,
                thumbnailBlob,
              );
            } catch (e) {
              console.warn(
                `Failed to fetch thumbnail, retrying (${tries + 1}/3): ${e}`,
              );
              await new Promise((resolve) => setTimeout(resolve, 2000));
              if (tries === 2) {
                throw new Error(
                  `Failed to fetch thumbnail after 3 tries: ${e}`,
                );
              }
            }
          }
        } catch (e) {
          throw new Error(`Thumbnail upload failed ${e}`);
        }
      }

      const newPost = await state.golivePost(title, now, thumbnail);

      if (!newPost?.uri || !newPost?.cid) {
        throw new Error(
          "Cannot read properties of undefined (reading 'uri' or 'cid')",
        );
      }

      const record: PlaceStreamLivestream.Record = {
        $type: "place.stream.livestream",
        title: title,
        url: streamplaceUrl,
        createdAt: new Date().toISOString(),
        post: {
          uri: newPost.uri,
          cid: newPost.cid,
        },
        thumb: thumbnail,
      };

      await state.pdsAgent.com.atproto.repo.createRecord({
        repo: did,
        collection: "place.stream.livestream",
        record,
      });
      set({
        newLivestream: {
          loading: false,
          error: null,
          record: record,
        },
      });
    } catch (error) {
      console.error("createLivestreamRecord rejected", error);
      set({
        newLivestream: {
          loading: false,
          error: error?.message ?? null,
          record: null,
        },
      });
    }
  },

  updateLivestreamRecord: async (title: string, livestream: any) => {
    set({
      newLivestream: {
        loading: true,
        error: null,
        record: null,
      },
    });
    try {
      const now = new Date();
      const state = get() as BlueskySlice;

      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
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

      const streamplaceUrl = get().url;
      const record: PlaceStreamLivestream.Record = {
        $type: "place.stream.livestream",
        title: title,
        url: streamplaceUrl,
        createdAt: new Date().toISOString(),
        post: oldRecordValue.post,
      };

      await state.pdsAgent.com.atproto.repo.putRecord({
        repo: did,
        collection: "place.stream.livestream",
        rkey,
        record,
      });
      set({
        newLivestream: {
          loading: false,
          error: null,
          record: record,
        },
      });
    } catch (error) {
      console.error("createLivestreamRecord rejected", error);
      set({
        newLivestream: {
          loading: false,
          error: error?.message ?? null,
          record: null,
        },
      });
    }
  },

  getChatProfileRecordFromPDS: async () => {
    set({
      chatProfile: {
        loading: true,
        error: null,
        profile: null,
      },
    });
    try {
      const state = get() as BlueskySlice;
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const res = await state.pdsAgent.com.atproto.repo.getRecord({
        repo: did,
        collection: "place.stream.chat.profile",
        rkey: "self",
      });
      if (!res.success) {
        throw new Error("Failed to get chat profile record");
      }

      if (PlaceStreamChatProfile.isRecord(res.data.value)) {
        set({
          chatProfile: {
            loading: false,
            error: null,
            profile: res.data.value,
          },
        });
      } else {
        console.log("not a record", res.data.value);
      }
    } catch (error) {
      console.error("getChatProfileRecordFromPDS error", error);
    }
  },

  createChatProfileRecord: async (red: number, green: number, blue: number) => {
    set({
      chatProfile: {
        loading: true,
        error: null,
        profile: null,
      },
    });
    try {
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }

      const chatProfile: PlaceStreamChatProfile.Record = {
        $type: "place.stream.chat.profile",
        color: {
          red: red,
          green: green,
          blue: blue,
        },
      };

      const res = await state.pdsAgent.com.atproto.repo.putRecord({
        repo: did,
        collection: "place.stream.chat.profile",
        record: chatProfile,
        rkey: "self",
      });
      if (!res.success) {
        throw new Error("Failed to create chat profile record");
      }
      set({
        chatProfile: {
          loading: false,
          error: null,
          profile: chatProfile,
        },
      });
    } catch (error) {
      console.error("createChatProfileRecord rejected", error);
      set({
        chatProfile: {
          loading: false,
          error: error?.message ?? null,
          profile: null,
        },
      });
    }
  },

  followUser: async (subjectDID: string) => {
    try {
      console.log("followUser pending");
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      await state.pdsAgent.follow(subjectDID);
      console.log("followUser fulfilled", { subjectDID });
    } catch (error) {
      console.error("followUser rejected", error);
    }
  },

  unfollowUser: async (subjectDID: string, followUri?: string) => {
    try {
      console.log("unfollowUser pending");
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }

      if (followUri) {
        await state.pdsAgent.deleteFollow(followUri);
      } else {
        const streamplaceUrl = get().url;
        const res = await fetch(
          `${streamplaceUrl}/xrpc/place.stream.graph.getFollowingUser?subjectDID=${encodeURIComponent(subjectDID)}&userDID=${encodeURIComponent(did)}`,
          {
            credentials: "include",
          },
        );
        const data = await res.json();

        if (!data.follow || !data.follow.uri) {
          throw new Error("Follow record not found");
        }

        await state.pdsAgent.deleteFollow(data.follow.uri);
      }

      console.log("unfollowUser fulfilled", { subjectDID });
    } catch (error) {
      console.error("unfollowUser rejected", error);
    }
  },

  getServerSettingsFromPDS: async () => {
    try {
      const state = get() as BlueskySlice;
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const streamplaceUrl = get().url;
      const u = new URL(streamplaceUrl);
      const res = await state.pdsAgent.com.atproto.repo.getRecord({
        repo: did,
        collection: "place.stream.server.settings",
        rkey: u.host,
      });
      if (!res.success) {
        throw new Error("Failed to get chat profile record");
      }

      if (PlaceStreamServerSettings.isRecord(res.data.value)) {
        set({
          serverSettings: res.data.value as PlaceStreamServerSettings.Record,
        });
      } else {
        console.log("not a record", res.data.value);
      }
    } catch (error) {
      console.error("getServerSettingsFromPDS rejected", error);
    }
  },

  createServerSettingsRecord: async (debugRecording: boolean) => {
    try {
      const state = get() as BlueskySlice;
      if (!state.pdsAgent) {
        throw new Error("No agent");
      }
      const did = state.oauthSession?.did;
      if (!did) {
        throw new Error("No DID");
      }
      const profile = state.profiles[did];
      if (!profile) {
        throw new Error("No profile");
      }
      const streamplaceUrl = get().url;
      const u = new URL(streamplaceUrl);
      const serverSettings: PlaceStreamServerSettings.Record = {
        $type: "place.stream.server.settings",
        debugRecording: debugRecording,
      };

      const res = await state.pdsAgent.com.atproto.repo.putRecord({
        repo: did,
        collection: "place.stream.server.settings",
        record: serverSettings,
        rkey: u.host,
      });
      if (!res.success) {
        throw new Error("Failed to create server settings record");
      }
      set({
        serverSettings: serverSettings,
      });
    } catch (error) {
      console.error("createServerSettingsRecord rejected", error);
    }
  },
});
