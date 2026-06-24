// Bluesky/ATProto authentication, profile, follow, and per-user record
// actions (chat profile, server settings). Ported from
// js/app/store/slices/blueskySlice.ts.
//
// Differences from the app source:
//   - storage is imported from "../../storage" (the web's localStorage
//     wrapper) instead of @streamplace/components.
//   - createOAuthClient is imported from the web's lib/oauth, which
//     returns a BrowserOAuthClient (the same type the app's web variant
//     surfaces from features/bluesky/oauthClientImport).
//   - clearQueryParams is a tiny inline helper instead of the app's
//     utils/clear-query-params, since the app util is React-Native-gated.
//   - The following actions are NOT ported in this commit and are
//     placeholders for later phases. They depend on libraries that are
//     not yet in js/web's deps:
//       - createStreamKeyRecord, clearStreamKeyRecord,
//         getStreamKeyRecords, deleteStreamKeyRecord (Phase 3 settings
//         / Phase 4 go-live; needs @atproto/crypto + viem).
//       - createLivestreamRecord, updateLivestreamRecord, golivePost,
//         and the uploadThumbnail helper (Phase 4 go-live).
//       - createBlockRecord (low priority; revisit if/when needed).
//     The corresponding state fields (streamKeysResponse, newLivestream,
//     newKey, etc.) are kept so the slice type stays symmetric with the
//     app's; they simply aren't mutated yet.
import { Agent } from "@atproto/api";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { OutputSchema } from "@atproto/api/dist/client/types/com/atproto/repo/listRecords";
import { OAuthSession } from "@atproto/oauth-client-browser";
import { getBrowserName } from "@streamplace/core";
import {
  PlaceStreamChatProfile,
  PlaceStreamLivestream,
  PlaceStreamServerSettings,
  StreamplaceAgent,
} from "streamplace";
import { StateCreator } from "zustand";
import createOAuthClient from "../../oauth";
import { storage } from "../../storage";
import { AppStore } from "../index";
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
  client: null | Awaited<ReturnType<typeof createOAuthClient>>;
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
  login: (handle: string, mode?: "popup" | "redirect") => Promise<void>;
  logout: () => Promise<void>;
  getProfile: (actor: string) => Promise<void>;
  getProfiles: (actors: string[]) => Promise<void>;
  oauthCallback: (url: string) => Promise<void>;
  setReturnRoute: (route: { name: string; params?: any } | null) => void;
  setLoginError: (error: string | null) => void;
  showLoginModal: boolean;
  openLoginModal: (returnRoute?: { name: string; params?: any }) => void;
  closeLoginModal: () => void;
  showPdsModal: boolean;
  openPdsModal: () => void;
  closePdsModal: () => void;
  // TODO(phase 3/4): stream-key + go-live + block actions. See comment
  // at the top of this file.
  golivePost: (
    text: string,
    now: Date,
    thumbnail?: any,
  ) => Promise<{ uri: string; cid: string }>;
  createBlockRecord: (subjectDID: string) => Promise<void>;
  createStreamKeyRecord: (store: boolean) => Promise<void>;
  clearStreamKeyRecord: () => void;
  getStreamKeyRecords: () => Promise<void>;
  deleteStreamKeyRecord: (
    rkey?: string,
    batchRkeys?: string[],
  ) => Promise<void>;
  setPDS: (pds: string) => Promise<void>;
  createLivestreamRecord: (
    title: string,
    customThumbnail?: Blob,
    activity?: PlaceStreamLivestream.Record["activity"],
    tags?: string[],
  ) => Promise<void>;
  updateLivestreamRecord: (
    title: string,
    livestream: any,
    activity?: PlaceStreamLivestream.Record["activity"],
    tags?: string[],
  ) => Promise<void>;
  getChatProfileRecordFromPDS: () => Promise<void>;
  createChatProfileRecord: (
    red: number,
    green: number,
    blue: number,
    selfLabels?: string[],
  ) => Promise<void>;
  followUser: (subjectDID: string) => Promise<void>;
  unfollowUser: (subjectDID: string, followUri?: string) => Promise<void>;
  getServerSettingsFromPDS: () => Promise<void>;
  createServerSettingsRecord: (debugRecording: boolean) => Promise<void>;
}

// Inline OAuth-callback URL scrubber. The app's `utils/clear-query-params`
// is Platform.OS-gated and short-circuits on native; the web always wants
// to scrub these.
function clearQueryParams(par: string[] = ["iss", "state", "code"]) {
  if (typeof document === "undefined") return;
  const u = new URL(document.location.href);
  if (u.search === "") return;
  const params = new URLSearchParams(u.search);
  par.forEach((p) => params.delete(p));
  u.search = params.toString();
  window.history.replaceState(null, "", u.toString());
}

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
  showPdsModal: false,
  notification: null,

  clearNotification: () => {
    clearQueryParams();
    set({ notification: null });
  },

  setReturnRoute: async (route: { name: string; params?: any } | null) => {
    if (route) {
      await storage.setItem("returnRoute", JSON.stringify(route));
    } else {
      await storage.removeItem("returnRoute");
    }
    set({ returnRoute: route });
  },

  openLoginModal: async (returnRoute?: { name: string; params?: any }) => {
    if (returnRoute) {
      await storage.setItem("returnRoute", JSON.stringify(returnRoute));
    }
    set({ showLoginModal: true, returnRoute: returnRoute || null });
  },

  closeLoginModal: () => {
    set({ showLoginModal: false });
  },

  setLoginError: (error) => {
    set((s) => ({ loginState: { ...s.loginState, error } }));
  },

  openPdsModal: () => {
    set({ showPdsModal: true });
  },

  closePdsModal: () => {
    set({ showPdsModal: false });
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
      ]);
      const did = maybeDIDs.find((d) => d !== null) || null;
      let session: OAuthSession | null = null;
      if (did) {
        try {
          session = await client.restore(did);
        } catch (e) {
          console.error("Error restoring session", e);
          await storage.removeItem(DID_KEY);
          await storage.removeItem("@@atproto/oauth-client-browser(sub)");
        }
      }
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

  login: async (handle: string, mode: "popup" | "redirect" = "popup") => {
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
      if (mode === "redirect") {
        // Full-page redirect to the PDS OAuth page. The user
        // authenticates, comes back to /login?code=..., the callback
        // runs, and the user is logged in. Used by the /login route,
        // which is the fallback for users on the full-page already
        // (or who got bounced here by the modal's popup-blocker
        // detection). The promise never resolves — the page is
        // navigating away.
        const url = await updatedState.client.authorize(handle, {});
        window.location.href = url.href;
        await new Promise<never>(() => {});
        return;
      }
      // mode === "popup": use the library's built-in popup flow. It
      // opens the popup synchronously (before authorize, to avoid
      // popup blockers), writes the OAuth state in sessionStorage, and
      // listens on a BroadcastChannel for the popup's initCallback to
      // send back the result. Resolves with the restored OAuthSession.
      const session = await updatedState.client.signInPopup(handle);
      await storage.setItem(DID_KEY, session.did);
      set({
        client: updatedState.client,
        oauthSession: session,
        pdsAgent: new StreamplaceAgent(session),
        authStatus: "loggedIn",
      });
    } catch (error: any) {
      console.error("login rejected", error, error?.cause);
      let message = error?.message || "unknown error";
      // The library's OAuthResponseError message is "OAuth unknown error"
      // when the PDS returns a non-OK token-exchange response without
      // standard `error` / `error_description` fields. In practice this
      // is almost always transient — the PDS is still processing the
      // previous session's revoke, rate-limiting, etc. Surface it as a
      // "try again in a moment" hint instead of a dead-end.
      if (message.startsWith("OAuth unknown error")) {
        message = "Sign-in failed. Please wait a moment and try again.";
      }
      set({
        loginState: {
          loading: false,
          error: message,
        },
        notification: {
          message,
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
      if (!url.includes("?")) {
        throw new Error("No query params");
      }
      const params = new URLSearchParams(url.split("?")[1]);
      if (!(params.has("code") && params.has("state") && params.has("iss"))) {
        if (params.has("error")) {
          const blueskySlice = get() as BlueskySlice;
          console.log("OAuth error params", {
            error: params.get("error"),
            error_description: params.get("error_description"),
          });
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
        // initCallback handles the popup handoff via BroadcastChannel:
        // when the state param is POPUP_STATE_PREFIX-prefixed (i.e. this
        // page was opened by signInPopup), initCallback sends the result
        // back to the parent and throws LoginContinuedInParentWindowError
        // after also calling window.close(). The route's countdown UI
        // shows briefly before the popup actually closes.
        const ret = await client.initCallback(params);
        if (!ret) {
          return;
        }
        await storage.setItem(DID_KEY, ret.session.did);
        set({
          client,
          oauthSession: ret.session,
          pdsAgent: new StreamplaceAgent(ret.session),
          authStatus: "loggedIn",
        });
      } catch (e: any) {
        // In the popup case, the library's initCallback sends the result
        // (success or error) to the parent via BroadcastChannel and
        // throws LoginContinuedInParentWindowError. The parent owns the
        // session state; the popup is just a pass-through. We don't
        // want a notification toast in a closing popup, and we don't
        // need to clobber the auth status beyond "loading -> not loading"
        // (which the route watches to start the closing countdown).
        if (
          typeof window !== "undefined" &&
          window.opener &&
          e?.code === "LOGIN_CONTINUED_IN_PARENT_WINDOW"
        ) {
          set({ authStatus: "loggedOut" });
          return;
        }

        let message = e.message;
        let cause = e.cause;
        while (cause) {
          message = `${message}: ${cause.message}`;
          cause = cause.cause;
        }
        // PDS token-exchange failure with no useful error fields —
        // almost always transient (rate limiting, revoke still
        // processing). Tell the user to try again in a moment.
        if (message.startsWith("OAuth unknown error")) {
          message = "Sign-in failed. Please wait a moment and try again.";
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
    } catch (error: any) {
      console.error("oauthCallback rejected", error);
      let message = error?.message || "authentication failed";
      if (message.startsWith("OAuth unknown error")) {
        message = "Sign-in failed. Please wait a moment and try again.";
      }
      set({
        authStatus: "loggedOut",
        notification: {
          message,
          type: "error",
        },
      });
    }
  },

  // TODO(phase 4): golivePost needs RichText from @atproto/api and is
  // called from createLivestreamRecord. Both port together.
  golivePost: async (_text: string, _now: Date, _thumbnail?: any) => {
    throw new Error("golivePost not yet ported (Phase 4)");
  },

  // TODO: revisit if/when block UI is added in web.
  createBlockRecord: async (_subjectDID: string) => {
    throw new Error("createBlockRecord not ported");
  },

  // TODO(phase 3/4): stream-key actions need @atproto/crypto + viem.
  // Generate-keypair code is also used by createLivestreamRecord for
  // blob uploads, so adding the deps unblocks both at once.
  createStreamKeyRecord: async (_store: boolean) => {
    const state = get() as BlueskySlice;
    if (!state.pdsAgent) throw new Error("No agent");
    const did = state.oauthSession?.did;
    if (!did) throw new Error("No DID");

    const { Secp256k1Keypair, bytesToMultibase } =
      await import("@atproto/crypto");
    const { privateKeyToAccount } = await import("viem/accounts");

    const keypair = await Secp256k1Keypair.create({ exportable: true });
    const exportedKey = await keypair.export();
    const didBytes = new TextEncoder().encode(did);
    const combinedKey = new Uint8Array([...exportedKey, ...didBytes]);
    const multibaseKey = bytesToMultibase(combinedKey, "base58btc");
    const hexKey = Array.from(exportedKey)
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    const account = privateKeyToAccount(`0x${hexKey}`);

    let userAgent = "";
    if (typeof navigator !== "undefined") {
      userAgent = navigator.userAgent;
    }

    const browserFamilyName = getBrowserName(userAgent);

    const record = {
      $type: "place.stream.key" as const,
      signingKey: keypair.did(),
      createdAt: new Date().toISOString(),
      createdBy: `Streamplace Web${browserFamilyName ? ` on ${browserFamilyName}` : ""}`,
    };

    await state.pdsAgent.com.atproto.repo.createRecord({
      repo: did,
      collection: "place.stream.key",
      record,
    });

    const newKey = {
      privateKey: multibaseKey,
      did: keypair.did(),
      address: account.address.toLowerCase(),
    };

    set({ newKey });
    // Refresh the list
    await get().getStreamKeyRecords();
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
    const state = get() as BlueskySlice;
    if (!state.pdsAgent) {
      set({
        streamKeysResponse: {
          loading: false,
          error: "No agent",
          records: null,
        },
      });
      return;
    }
    const did = state.oauthSession?.did;
    if (!did) {
      set({
        streamKeysResponse: {
          loading: false,
          error: "No DID",
          records: null,
        },
      });
      return;
    }
    try {
      // Fetch all keys (paginate through results)
      let allRecords: any[] = [];
      let cursor: string | undefined;
      do {
        const result = await state.pdsAgent.com.atproto.repo.listRecords({
          repo: did,
          collection: "place.stream.key",
          limit: 100,
          ...(cursor ? { cursor } : {}),
        });
        allRecords = allRecords.concat(result.data.records);
        cursor = result.data.cursor;
      } while (cursor);

      set({
        streamKeysResponse: {
          loading: false,
          error: null,
          records: { records: allRecords },
        },
      });
    } catch (error: any) {
      set({
        streamKeysResponse: {
          loading: false,
          error: error?.message ?? null,
          records: null,
        },
      });
    }
  },
  deleteStreamKeyRecord: async (rkey?: string, batchRkeys?: string[]) => {
    // If batchRkeys is provided, it takes precedence over rkey and deletes all keys in the array.
    // If not, it deletes the single rkey. If neither is provided, it throws an error.
    if (!rkey && !batchRkeys) {
      throw new Error("No rkey(s) provided for deletion");
    }
    set({ isDeletingKey: true });
    const state = get() as BlueskySlice;
    if (!state.pdsAgent) {
      set({ isDeletingKey: false });
      throw new Error("No agent");
    }
    const did = state.oauthSession?.did;
    if (!did) {
      set({ isDeletingKey: false });
      throw new Error("No DID");
    }
    try {
      const keysToDelete = batchRkeys ?? [rkey];

      if (keysToDelete.length === 1) {
        // Single delete
        await state.pdsAgent.com.atproto.repo.deleteRecord({
          repo: did,
          collection: "place.stream.key",
          rkey: keysToDelete[0] as string,
        });
      } else {
        // Batch delete via applyWrites
        await state.pdsAgent.com.atproto.repo.applyWrites({
          repo: did,
          // TODO: type this properly
          writes: keysToDelete.map((k) => ({
            $type: "com.atproto.repo.applyWrites#delete" as const,
            collection: "place.stream.key",
            rkey: k,
          })) as any,
        });
      }

      const deletedSet = new Set(keysToDelete);
      const records = state.streamKeysResponse.records
        ? state.streamKeysResponse.records.records.filter(
            (r) => !deletedSet.has(r.uri.split("/").pop() as string),
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
      set({ isDeletingKey: false });
      throw error;
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
      set({
        pds: {
          ...(get() as BlueskySlice).pds,
          loading: false,
          url: pds,
        },
      });
    } catch (error: any) {
      set({
        pds: {
          ...(get() as BlueskySlice).pds,
          loading: false,
          error: error?.message ?? null,
        },
      });
    }
  },

  // TODO(phase 4): go-live. Needs golivePost (RichText) + blob upload
  // helper. Both are pure @atproto/api code, no extra deps.
  createLivestreamRecord: async () => {
    throw new Error("createLivestreamRecord not yet ported (Phase 4)");
  },
  updateLivestreamRecord: async () => {
    throw new Error("updateLivestreamRecord not yet ported (Phase 4)");
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
    } catch (error: any) {
      set({
        chatProfile: {
          loading: false,
          error: error?.message ?? "Failed to get chat profile",
          profile: null,
        },
      });
    }
  },

  createChatProfileRecord: async (
    red: number,
    green: number,
    blue: number,
    selfLabels?: string[],
  ) => {
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

      const existingProfile = (get() as BlueskySlice).chatProfile?.profile;
      const chatProfile: PlaceStreamChatProfile.Record = {
        ...existingProfile,
        $type: "place.stream.chat.profile",
        color: {
          red: red,
          green: green,
          blue: blue,
        },
        selfLabels: selfLabels,
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
    const state = get() as BlueskySlice;
    if (!state.pdsAgent) {
      throw new Error("No agent");
    }
    const did = state.oauthSession?.did;
    if (!did) {
      throw new Error("No DID");
    }
    await state.pdsAgent.follow(subjectDID);
  },

  unfollowUser: async (subjectDID: string, followUri?: string) => {
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
  },

  getServerSettingsFromPDS: async () => {
    const state = get() as BlueskySlice;
    const did = state.oauthSession?.did;
    if (!did) {
      throw new Error("No DID");
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
      throw new Error("Failed to get server settings record");
    }

    if (PlaceStreamServerSettings.isRecord(res.data.value)) {
      set({
        serverSettings: res.data.value as PlaceStreamServerSettings.Record,
      });
    } else {
      console.log("not a record", res.data.value);
    }
  },

  createServerSettingsRecord: async (debugRecording: boolean) => {
    const state = get() as BlueskySlice;
    if (!state.pdsAgent) {
      throw new Error("No agent");
    }
    const did = state.oauthSession?.did;
    if (!did) {
      throw new Error("No DID");
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
  },
});
