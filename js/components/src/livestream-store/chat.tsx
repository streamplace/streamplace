import { ChatMessageViewHydrated, PlaceStreamDefs } from "streamplace";
import { LivestreamState } from "./livestream-state";

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
