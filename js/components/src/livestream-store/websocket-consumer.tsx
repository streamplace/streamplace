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
import { findProblems } from "./problems";

const MAX_RECENT_SEGMENTS = 10;

export const handleWebSocketMessages = (
  state: LivestreamState,
  messages: any[],
): LivestreamState => {
  for (let message of messages) {
    if (message.$type === "place.stream.error") {
      state = {
        ...state,
        problems: [
          ...state.problems,
          {
            code: message.code,
            message: message.message,
            severity: "error",
          },
        ],
      };
    } else {
      if (!state.websocketConnected) {
        state = {
          ...state,
          websocketConnected: true,
        };
      }

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
        message = message as PlaceStreamLivestream.ViewerCount;
        state = {
          ...state,
          viewers: message.count,
        };
      } else if (PlaceStreamChatDefs.isMessageView(message)) {
        message = message as PlaceStreamChatDefs.MessageView;
        // Explicitly map MessageView to MessageViewHydrated
        const hydrated: ChatMessageViewHydrated = {
          uri: message.uri,
          cid: message.cid,
          author: message.author,
          record: message.record as PlaceStreamChatMessage.Record,
          indexedAt: message.indexedAt,
          chatProfile: (message as any).chatProfile,
          replyTo: (message as any).replyTo,
          deleted: message.deleted,
        };
        state = reduceChat(state, [hydrated], [], []);
      } else if (PlaceStreamSegment.isRecord(message)) {
        const newRecentSegments = [...state.recentSegments];
        newRecentSegments.unshift(message);
        if (newRecentSegments.length > MAX_RECENT_SEGMENTS) {
          newRecentSegments.pop();
        }
        state = {
          ...state,
          segment: message as PlaceStreamSegment.Record,
          recentSegments: newRecentSegments,
          problems: findProblems(newRecentSegments),
          hasReceivedSegment: true,
        };
      } else if (PlaceStreamDefs.isBlockView(message)) {
        const block = message as PlaceStreamDefs.BlockView;
        state = reduceChat(state, [], [block], []);
      } else if (PlaceStreamDefs.isRenditions(message)) {
        message = message as PlaceStreamDefs.Renditions;
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
  }
  return reduceChat(state, [], [], []);
};
