import { RichText } from "@atproto/api";
import {
  isLink,
  isMention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
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
  return (message: ChatMessageViewHydrated | null) => {
    store.setState({ replyToMessage: message });
  };
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
    rt.detectFacetsWithoutResolution();

    const record: PlaceStreamChatMessage.Record = {
      text: msg.text,
      createdAt: new Date().toISOString(),
      streamer: streamerProfile.did,
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
      ...(rt.facets && rt.facets.length > 0
        ? {
            facets: rt.facets.map((facet) => ({
              index: facet.index,
              features: facet.features
                .filter(
                  (feature) =>
                    feature.$type === "app.bsky.richtext.facet#link" ||
                    feature.$type === "app.bsky.richtext.facet#mention",
                )
                .map((feature) => {
                  if (isLink(feature)) {
                    return {
                      $type: "app.bsky.richtext.facet#link",
                      uri: feature.uri,
                    };
                  } else if (isMention(feature)) {
                    return {
                      $type: "app.bsky.richtext.facet#mention",
                      did: feature.did,
                    };
                  } else {
                    throw new Error("invalid code path");
                  }
                }),
            })),
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

    state = reduceChat(state, [localChat], []);
    store.setState(state);

    await pdsAgent.com.atproto.repo.createRecord({
      repo: userDID,
      collection: "place.stream.chat.message",
      record,
    });
  };
};

export const reduceChat = (
  state: LivestreamState,
  messages: ChatMessageViewHydrated[],
  blocks: PlaceStreamDefs.BlockView[],
): LivestreamState => {
  state = { ...state } as LivestreamState;
  const newChat: { [key: string]: ChatMessageViewHydrated } = {
    ...state.chatIndex,
  };

  // Add new messages
  for (let message of messages) {
    const date = new Date(message.record.createdAt);
    const key = `${date.getTime()}-${message.uri}`;

    // Remove existing local message matching the server one
    if (!message.uri.startsWith("local-")) {
      const existingLocalMessageKey = Object.keys(newChat).find((k) => {
        const msg = newChat[k];
        return (
          msg.uri.startsWith("local-") &&
          msg.record.text === message.record.text &&
          msg.author.did === message.author.did
        );
      });

      if (existingLocalMessageKey) {
        delete newChat[existingLocalMessageKey];
      }
    }

    // Handle reply information for local-first messages
    if (message.record.reply) {
      const reply = message.record.reply as {
        parent?: { uri: string; cid: string };
        root?: { uri: string; cid: string };
      };

      const parentUri = reply?.parent?.uri || reply?.root?.uri;

      if (parentUri) {
        // First try to find the parent message in our chat
        const parentMsgKey = Object.keys(newChat).find(
          (k) => newChat[k].uri === parentUri,
        );

        if (parentMsgKey) {
          // Found the parent message, add its info to our message
          const parentMsg = newChat[parentMsgKey];
          message = {
            ...message,
            replyTo: {
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

    newChat[key] = message;
  }

  for (const block of blocks) {
    for (const [k, v] of Object.entries(newChat)) {
      if (v.author.did === block.record.subject) {
        delete newChat[k];
      }
    }
  }

  const newChatList = Object.keys(newChat)
    .sort((a, b) => (a > b ? 1 : -1))
    .map((key) => newChat[key]);

  return {
    ...state,
    chatIndex: newChat,
    chat: newChatList,
  };
};
