import { useContext } from "react";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PlaceStreamChatDefs,
  PlaceStreamChatMessage,
  PlaceStreamDefs,
  PlaceStreamLivestream,
  PlaceStreamSegment,
} from "streamplace";
import { createStore, StoreApi, useStore } from "zustand";
import { reduceChat } from "./chat";
import { LivestreamContext } from "./context";
import { LivestreamState } from "./livestream-state";

export type LivestreamStore = StoreApi<LivestreamState>;

export const makeLivestreamStore = (): StoreApi<LivestreamState> => {
  return createStore<LivestreamState>()((set) => ({
    chatIndex: {},
    chat: [],
    handleWebSocketMessages: (messages: any[]) =>
      set((state) => handleWebSocketMessages(state, messages)),
    livestream: null,
    viewers: null,
    segment: null,
    renditions: [],
  }));
};

const handleWebSocketMessages = (
  state: LivestreamState,
  messages: any[],
): LivestreamState => {
  for (const message of messages) {
    if (PlaceStreamLivestream.isLivestreamView(message)) {
      state = {
        ...state,
        livestream: message as LivestreamViewHydrated,
      };
    } else if (PlaceStreamLivestream.isViewerCount(message)) {
      state = {
        ...state,
        viewers: message.count,
      };
    } else if (PlaceStreamChatDefs.isMessageView(message)) {
      // Explicitly map MessageView to MessageViewHydrated
      const hydrated: ChatMessageViewHydrated = {
        uri: message.uri,
        cid: message.cid,
        author: message.author,
        record: message.record as PlaceStreamChatMessage.Record,
        indexedAt: message.indexedAt,
        chatProfile: (message as any).chatProfile,
        replyTo: (message as any).replyTo,
      };
      state = reduceChat(state, [hydrated], []);
    } else if (PlaceStreamSegment.isRecord(message)) {
      state = {
        ...state,
        segment: message as PlaceStreamSegment.Record,
      };
    } else if (PlaceStreamDefs.isBlockView(message)) {
      const block = message as PlaceStreamDefs.BlockView;
      state = reduceChat(state, [], [block]);
    } else if (PlaceStreamDefs.isRenditions(message)) {
      state = {
        ...state,
        renditions: message.renditions,
      };
    }
  }
  return reduceChat(state, [], []);
};

export function useLivestreamStore<U>(
  selector: (state: LivestreamState) => U,
): U {
  const context = useContext(LivestreamContext);
  if (!context) {
    throw new Error(
      "useLivestreamStore must be used within a LivestreamProvider",
    );
  }
  return useStore(context.store, selector);
}

export const useChat = () => useLivestreamStore((x) => x.chat);

export const useHandleWebsocketMessages = () =>
  useLivestreamStore((x) => x.handleWebSocketMessages);
