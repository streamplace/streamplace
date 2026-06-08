import { AppBskyActorDefs } from "@atproto/api";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PinnedRecordViewHydrated,
  place,
} from "streamplace";
import { formatHandleWithAt } from "../lib/format-handle";
import { SystemMessages } from "../lib/system-messages";
import { reduceChat } from "./chat-reducer";
import { findProblems } from "./problems";
import { LivestreamState } from "./state";

const MAX_RECENT_SEGMENTS = 10;

export const handleWebSocketMessages = (
  state: LivestreamState,
  messages: any[],
): LivestreamState => {
  for (let message of messages) {
    console.log("Received WebSocket message:", message);
    if (message.$type === "place.stream.error") {
      // Dedupe by code: the server re-emits the same error on every offending
      // segment (e.g. a stream that keeps reconnecting over the bitrate limit),
      // so replace any existing problem of this code rather than stacking copies.
      state = {
        ...state,
        problems: [
          ...state.problems.filter((p) => p.code !== message.code),
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

      if (place.stream.livestream.livestreamView.isTypeOf(message)) {
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
      } else if (place.stream.livestream.viewerCount.isTypeOf(message)) {
        message = message as place.stream.livestream.ViewerCount;
        state = {
          ...state,
          viewers: message.count,
        };
      } else if (place.stream.chat.defs.messageView.isTypeOf(message)) {
        message = message as place.stream.chat.defs.MessageView;
        // Explicitly map MessageView to MessageViewHydrated
        const hydrated: ChatMessageViewHydrated = {
          uri: message.uri,
          cid: message.cid,
          author: message.author,
          record: message.record as place.stream.chat.message.Main,
          indexedAt: message.indexedAt,
          chatProfile: (message as any).chatProfile,
          replyTo: (message as any).replyTo,
          deleted: message.deleted,
          badges: message.badges,
        };
        state = reduceChat(state, [hydrated], [], []);
      } else if (place.stream.segment.$isTypeOf(message)) {
        const newRecentSegments = [...state.recentSegments];
        newRecentSegments.unshift(message);
        if (newRecentSegments.length > MAX_RECENT_SEGMENTS) {
          newRecentSegments.pop();
        }
        state = {
          ...state,
          segment: message as place.stream.segment.Main,
          recentSegments: newRecentSegments,
          problems: findProblems(newRecentSegments),
          hasReceivedSegment: true,
        };
      } else if (place.stream.defs.blockView.isTypeOf(message)) {
        const block = message as place.stream.defs.BlockView;
        state = reduceChat(state, [], [block], []);
      } else if (place.stream.defs.renditions.isTypeOf(message)) {
        message = message as place.stream.defs.Renditions;
        state = {
          ...state,
          renditions: message.renditions,
        };
      } else if (AppBskyActorDefs.isProfileViewBasic(message)) {
        state = {
          ...state,
          profile: message,
        };
      } else if (place.stream.chat.gate.$isTypeOf(message)) {
        const hideRecord = message as place.stream.chat.gate.Main;
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
      } else if (place.stream.chat.defs.pinnedRecordView.isTypeOf(message)) {
        const pinnedView = message as PinnedRecordViewHydrated;
        state = {
          ...state,
          pinnedComment: pinnedView,
        };
      } else if (
        (message as any).$type === "place.stream.chat.pinnedRecord" &&
        (message as any).deleted === true
      ) {
        state = {
          ...state,
          pinnedComment: null,
        };
      } else if (place.stream.live.teleport.$isTypeOf(message)) {
        const teleportRecord = message as place.stream.live.teleport.Main;
        state = {
          ...state,
          activeTeleport: teleportRecord,
        };
      } else if (place.stream.livestream.teleportArrival.isTypeOf(message)) {
        // teleport has succeeded, we are now at the target stream
        const arrival = message as place.stream.livestream.TeleportArrival;

        // add the teleporter's chat profile to the authors cache FIRST so mention rendering works
        if (arrival.chatProfile && arrival.source.did) {
          state = {
            ...state,
            authors: {
              ...state.authors,
              [arrival.source.did]: arrival.chatProfile,
            },
          };
        }

        const systemMessage = SystemMessages.teleportArrival(
          formatHandleWithAt(arrival.source),
          arrival.source.did,
          arrival.viewerCount,
          arrival.chatProfile,
        );
        // set proper times
        systemMessage.indexedAt = arrival.startsAt;
        systemMessage.record.createdAt = arrival.startsAt;

        state = reduceChat(state, [systemMessage], []);
      } else if (place.stream.livestream.teleportCanceled.isTypeOf(message)) {
        // teleport was canceled (deleted or denied)
        state = {
          ...state,
          activeTeleport: null,
          activeTeleportUri: null,
        };
      }
    }
  }
  return reduceChat(state, [], [], []);
};
