// Pure chat reducer. No React imports.
//
// Takes a LivestreamState plus a delta (new messages, blocks, hidden URIs)
// and returns a new state. Used by the websocket consumer, the React hooks
// (in @streamplace/components), and any other consumer that wants to apply
// chat updates to a LivestreamStore.
import { ChatMessageViewHydrated, place } from "streamplace";
import { LivestreamState } from "./state";

export type NewChatMessage = {
  text: string;
  reply?: {
    cid: string;
    uri: string;
  };
};

const buildSortedChatList = (
  chatIndex: { [key: string]: ChatMessageViewHydrated },
  _existingChatList: ChatMessageViewHydrated[],
  _newMessages: { key: string; message: ChatMessageViewHydrated }[],
  removedKeys: Set<string>,
): ChatMessageViewHydrated[] => {
  const sortedKeys = Object.keys(chatIndex).sort((a, b) => {
    const aTime = parseInt(a.split("-")[0], 10);
    const bTime = parseInt(b.split("-")[0], 10);
    return aTime - bTime;
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

export const reduceChat = (
  state: LivestreamState,
  newMessages: ChatMessageViewHydrated[],
  blocks: place.stream.defs.BlockView[],
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
      // find and remove the message from the index
      for (const [key, message] of Object.entries(newChatIndex)) {
        if (message.uri === msg.uri) {
          delete newChatIndex[key];
          removedKeys.add(key);
        }
      }
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
