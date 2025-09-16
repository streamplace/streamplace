import { ComAtprotoModerationCreateReport, RichText } from "@atproto/api";
import { useCallback } from "react";
import {
  ChatMessageViewHydrated,
  PlaceStreamChatMessage,
  PlaceStreamDefs,
} from "streamplace";
import { useChatProfile, useDID, useHandle } from "../streamplace-store";
import { usePDSAgent } from "../streamplace-store/xrpc";
import { LivestreamState } from "./livestream-state";
import { getStoreFromContext, useLivestreamStore } from "./livestream-store";

export const useReplyToMessage = () =>
  useLivestreamStore((state) => state.replyToMessage);

export const useSetReplyToMessage = () => {
  const store = getStoreFromContext();
  return useCallback(
    (message: ChatMessageViewHydrated | null) => {
      store.setState({ replyToMessage: message });
    },
    [store],
  );
};

export const usePendingHides = () =>
  useLivestreamStore((state) => state.pendingHides);

export const useAddPendingHide = () => {
  const store = getStoreFromContext();
  return useCallback(
    (messageUri: string) => {
      const state = store.getState();
      if (!state.pendingHides.includes(messageUri)) {
        const newPendingHides = [...state.pendingHides, messageUri];
        const newState = reduceChat(state, [], [], [messageUri]);
        store.setState({
          ...newState,
          pendingHides: newPendingHides,
        });
      }
    },
    [store],
  );
};

export type NewChatMessage = {
  text: string;
  reply?: {
    cid: string;
    uri: string;
  };
};

export const useCreateChatMessage = () => {
  const pdsAgent = usePDSAgent();
  const store = getStoreFromContext();
  const userDID = useDID();
  const userHandle = useHandle();
  const chatProfile = useChatProfile();

  return async (msg: NewChatMessage) => {
    if (!pdsAgent || !userDID) {
      throw new Error("No PDS agent or user DID found");
    }

    let state = store.getState();

    const streamerProfile = state.profile;

    if (!streamerProfile) {
      throw new Error("Profile not found");
    }

    const rt = new RichText({ text: msg.text });
    await rt.detectFacets(pdsAgent);

    // filter out any facets that aren't in the allowed list
    rt.facets = rt.facets?.filter((facet) => {
      return (
        // if all features are in the allowed list
        facet.features.every((feature) =>
          [
            "app.bsky.richtext.facet#link",
            "app.bsky.richtext.facet#mention",
          ].includes(feature.$type),
        )
      );
    });

    const record: PlaceStreamChatMessage.Record = {
      $type: "place.stream.chat.message",
      text: msg.text,
      createdAt: new Date().toISOString(),
      streamer: streamerProfile.did,
      facets: rt.facets as PlaceStreamChatMessage.Record["facets"],
      ...(msg.reply
        ? {
            reply: {
              root: {
                cid: msg.reply.cid,
                uri: msg.reply.uri,
              },
              parent: {
                cid: msg.reply.cid,
                uri: msg.reply.uri,
              },
            },
          }
        : {}),
    };

    const localChat: ChatMessageViewHydrated = {
      uri: `local-${Date.now()}`,
      cid: "",
      author: {
        did: userDID,
        handle: userHandle || userDID,
      },
      record: record,
      indexedAt: new Date().toISOString(),
      chatProfile: chatProfile || undefined,
    };

    state = reduceChat(state, [localChat], [], []);
    store.setState(state);

    await pdsAgent.com.atproto.repo.createRecord({
      repo: userDID,
      collection: "place.stream.chat.message",
      record,
    });
  };
};

export const useDeleteChatMessage = () => {
  const pdsAgent = usePDSAgent();
  if (!pdsAgent) {
    throw new Error("No PDS agent found");
  }
  const userDID = useDID();
  if (!userDID) {
    throw new Error("No user DID found");
  }
  return async (uri: string) => {
    const rkey = uri.split("/").pop();
    if (!rkey) {
      throw new Error("No rkey found");
    }
    return await pdsAgent.com.atproto.repo.deleteRecord({
      repo: userDID,
      collection: "place.stream.chat.message",
      rkey: rkey,
    });
  };
};

const buildSortedChatList = (
  chatIndex: { [key: string]: ChatMessageViewHydrated },
  existingChatList: ChatMessageViewHydrated[],
  newMessages: { key: string; message: ChatMessageViewHydrated }[],
  removedKeys: Set<string>,
): ChatMessageViewHydrated[] => {
  const sortedKeys = Object.keys(chatIndex).sort((a, b) => {
    const aTime = parseInt(a.split("-")[0], 10);
    const bTime = parseInt(b.split("-")[0], 10);
    return bTime - aTime;
  });
  return sortedKeys
    .map((key) => chatIndex[key])
    .filter((msg) => !removedKeys.has(msg.uri));
};

const profileIsDifferent = (
  newProfile: ChatMessageViewHydrated["chatProfile"],
  oldProfile: ChatMessageViewHydrated["chatProfile"],
) => {
  if (!oldProfile) {
    return true;
  }
  if (!newProfile) {
    return false;
  }
  if (!oldProfile.color) {
    return true;
  }
  if (!newProfile.color) {
    // idk. shouldn't happen.
    return false;
  }
  const { red: newRed, green: newGreen, blue: newBlue } = newProfile.color;
  const { red: oldRed, green: oldGreen, blue: oldBlue } = oldProfile.color;
  return newRed !== oldRed || newGreen !== oldGreen || newBlue !== oldBlue;
};

export const reduceChatIncremental = (
  state: LivestreamState,
  newMessages: ChatMessageViewHydrated[],
  blocks: PlaceStreamDefs.BlockView[],
  hideUris: string[] = [],
): LivestreamState => {
  if (
    newMessages.length === 0 &&
    blocks.length === 0 &&
    hideUris.length === 0
  ) {
    return state;
  }

  const newChatIndex = { ...state.chatIndex };
  const newAuthors = { ...state.authors };
  let hasChanges = false;
  const removedKeys = new Set<string>();

  for (const msg of newMessages) {
    if (msg.deleted) {
      hasChanges = true;
      removedKeys.add(msg.uri);
    }
  }
  newMessages = newMessages.filter((msg) => msg.deleted !== true);

  // handle blocks
  if (blocks.length > 0) {
    const blockedDIDs = new Set(blocks.map((block) => block.record.subject));
    for (const [key, message] of Object.entries(newChatIndex)) {
      if (blockedDIDs.has(message.author.did)) {
        delete newChatIndex[key];
        removedKeys.add(key);
        hasChanges = true;
      }
    }
  }

  if (hideUris.length > 0) {
    for (const [key, message] of Object.entries(newChatIndex)) {
      if (hideUris.includes(message.uri)) {
        delete newChatIndex[key];
        removedKeys.add(key);
        hasChanges = true;
      }
    }
  }

  const messagesToAdd: { key: string; message: ChatMessageViewHydrated }[] = [];

  for (const message of newMessages) {
    // don't worry about messages that will be hidden
    if (state.pendingHides.includes(message.uri)) {
      continue;
    }

    const date = new Date(message.record.createdAt);
    const key = `${date.getTime()}-${message.uri}`;

    // only change the ref if the profile is different to avoid re-renders elsewhere
    if (
      profileIsDifferent(message.chatProfile, newAuthors[message.author.did])
    ) {
      newAuthors[message.author.did] = message.chatProfile;
    }

    // skip messages we already have
    if (newChatIndex[key] && newChatIndex[key].uri === message.uri) {
      continue;
    }

    // if we have a local message, replace it with the new one
    if (!message.uri.startsWith("local-")) {
      const existingLocalKey = Object.keys(newChatIndex).find((k) => {
        const msg = newChatIndex[k];
        return (
          msg.uri.startsWith("local-") &&
          msg.record.text === message.record.text &&
          msg.author.did === message.author.did &&
          Math.abs(new Date(msg.record.createdAt).getTime() - date.getTime()) <
            10000 // Within 10 seconds
        );
      });

      if (existingLocalKey) {
        delete newChatIndex[existingLocalKey];
        removedKeys.add(existingLocalKey);
        hasChanges = true;
      }
    }

    // add reply info
    let processedMessage = message;
    if (message.record.reply) {
      const reply = message.record.reply as {
        parent?: { uri: string; cid: string };
        root?: { uri: string; cid: string };
      };

      const parentUri = reply?.parent?.uri || reply?.root?.uri;
      if (parentUri) {
        const parentMsgKey = Object.keys(newChatIndex).find(
          (k) => newChatIndex[k].uri === parentUri,
        );

        if (parentMsgKey) {
          const parentMsg = newChatIndex[parentMsgKey];
          // Don't allow replies to system messages
          if (parentMsg.author.did !== "did:sys:system") {
            processedMessage = {
              ...message,
              replyTo: {
                $type: "place.stream.chat.defs#messageView",
                cid: parentMsg.cid,
                uri: parentMsg.uri,
                author: parentMsg.author,
                record: parentMsg.record,
                chatProfile: parentMsg.chatProfile,
                indexedAt: parentMsg.indexedAt,
              },
            };
          }
        }
      }
    }

    messagesToAdd.push({ key, message: processedMessage });
    hasChanges = true;
  }

  // Add new messages to index
  for (const { key, message } of messagesToAdd) {
    newChatIndex[key] = message;
  }

  // only rebuild if we have changes
  if (!hasChanges) {
    return state;
  }

  // Build the new sorted chat list efficiently
  const newChatList = buildSortedChatList(
    newChatIndex,
    state.chat,
    messagesToAdd,
    removedKeys,
  );

  // Clean up pendingHides - remove URIs that we've now processed
  let newPendingHides = state.pendingHides;
  if (hideUris.length > 0) {
    newPendingHides = state.pendingHides.filter(
      (uri) => !hideUris.includes(uri),
    );
  }

  return {
    ...state,
    authors: newAuthors,
    chatIndex: newChatIndex,
    chat: newChatList,
    pendingHides: newPendingHides,
  };
};

export const useSubmitReport = () => {
  const pdsAgent = usePDSAgent();
  const userDID = useDID();

  return useCallback(
    async (
      subject: ComAtprotoModerationCreateReport.InputSchema["subject"],
      reasonType: string,
      reason?: string,
      // no clue about this
      moderationSvcDid: string = "did:web:stream.place",
    ) => {
      if (!pdsAgent || !userDID) {
        throw new Error("No PDS agent or user DID found");
      }

      try {
        const response = await pdsAgent.com.atproto.moderation.createReport(
          {
            reasonType,
            reason,
            subject: subject,
          },
          {
            headers: {
              // "atproto-proxy": `${userDID}#atproto_labeler`,
            },
          },
        );

        return response;
      } catch (error) {
        console.error("Failed to submit report:", error);
        throw error;
      }
    },
    [pdsAgent, userDID],
  );
};

export const useReportChatMessage = () => {
  const submitReport = useSubmitReport();

  return useCallback(
    async (
      message: ChatMessageViewHydrated,
      reasonType: string,
      reason?: string,
    ) => {
      const reportSubject = {
        $type: "com.atproto.repo.strongRef",
        uri: message.uri,
        cid: message.cid,
      };

      return await submitReport(reportSubject, reasonType, reason);
    },
    [submitReport],
  );
};

export const reduceChat = reduceChatIncremental;
