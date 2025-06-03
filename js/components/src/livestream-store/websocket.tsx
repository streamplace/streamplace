import { AppBskyActorDefs } from "@atproto/api";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PlaceStreamChatDefs,
  PlaceStreamChatMessage,
  PlaceStreamDefs,
  PlaceStreamLivestream,
  PlaceStreamSegment,
} from "streamplace";
import { reduceChat } from "./chat";
import { LivestreamState } from "./livestream-state";

export const handleWebSocketMessages = (
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
    } else if (AppBskyActorDefs.isProfileViewBasic(message)) {
      state = {
        ...state,
        profile: message,
      };
    }
  }
  return reduceChat(state, [], []);
};
