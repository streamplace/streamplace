import { AppBskyActorDefs } from "@atproto/api";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PlaceStreamChatDefs,
  PlaceStreamChatGate,
  PlaceStreamChatMessage,
  PlaceStreamDefs,
  PlaceStreamLivestream,
  PlaceStreamSegment,
} from "streamplace";
import { SystemMessages } from "../lib/system-messages";
import { reduceChat } from "./chat";
import { LivestreamState } from "./livestream-state";

export const handleWebSocketMessages = (
  state: LivestreamState,
  messages: any[],
): LivestreamState => {
  for (const message of messages) {
    if (PlaceStreamLivestream.isLivestreamView(message)) {
      const newLivestream = message as LivestreamViewHydrated;
      const oldLivestream = state.livestream;

      // check if this is actually new
      if (!oldLivestream || oldLivestream.uri !== newLivestream.uri) {
        const streamTitle = newLivestream.record.title || "something cool!";
        const systemMessage = SystemMessages.streamStart(streamTitle);
        // set proper times
        systemMessage.indexedAt = newLivestream.indexedAt;
        systemMessage.record.createdAt = newLivestream.record.createdAt;

        state = reduceChat(state, [systemMessage], []);
      }

      state = {
        ...state,
        livestream: newLivestream,
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
      state = reduceChat(state, [hydrated], [], []);
    } else if (PlaceStreamSegment.isRecord(message)) {
      state = {
        ...state,
        segment: message as PlaceStreamSegment.Record,
      };
    } else if (PlaceStreamDefs.isBlockView(message)) {
      const block = message as PlaceStreamDefs.BlockView;
      state = reduceChat(state, [], [block], []);
    } else if (PlaceStreamDefs.isRenditions(message)) {
      state = {
        ...state,
        renditions: message.renditions,
      };
    } else if (AppBskyActorDefs.isProfileViewBasic(message)) {
      state = {
        ...state,
        profile: message,
      };
    } else if (PlaceStreamChatGate.isRecord(message)) {
      const hideRecord = message as PlaceStreamChatGate.Record;
      const hiddenMessageUri = hideRecord.hiddenMessage;
      const newPendingHides = [...state.pendingHides];
      if (!newPendingHides.includes(hiddenMessageUri)) {
        newPendingHides.push(hiddenMessageUri);
      }

      state = {
        ...state,
        pendingHides: newPendingHides,
      };
      state = reduceChat(state, [], [], [hiddenMessageUri]);
    }
  }
  return reduceChat(state, [], [], []);
};
