import { AppBskyActorDefs } from "@atproto/api";
import {
  ChatMessageViewHydrated,
  LivestreamViewHydrated,
  PinnedRecordViewHydrated,
  PlaceStreamChatDefs,
  PlaceStreamChatGate,
  PlaceStreamChatMessage,
  PlaceStreamDefs,
  PlaceStreamLivestream,
  PlaceStreamLiveTeleport,
  PlaceStreamSegment,
} from "streamplace";
import { SystemMessages } from "../lib/system-messages";
import { emoteImageUrl } from "../utils/did";
import { formatHandleWithAt } from "../utils/format-handle";
import { reduceChat } from "./chat";
import { EmoteView, LivestreamState } from "./livestream-state";
import { findProblems } from "./problems";

const MAX_RECENT_SEGMENTS = 10;

function exchangeEmoteRefs(
  message: ChatMessageViewHydrated,
  emoteCache: { [aturi: string]: EmoteView },
): ChatMessageViewHydrated {
  if (!message.record.facets) return message;

  const newFacets = message.record.facets.map((facet) => ({
    ...facet,
    features: facet.features.map((feature) => {
      if (feature.$type === "place.stream.richtext.facet#emote") {
        const emote = feature as {
          name: string;
          ref?: { uri: string; cid: string };
        };
        if (emote.ref) {
          const cached = emoteCache[emote.ref.uri];
          if (cached) {
            return {
              $type: "place.stream.richtext.facet#emote",
              name: cached.name,
              ref: cached,
            } as any;
          }
        }
      }
      return feature;
    }),
  }));

  return {
    ...message,
    record: {
      ...message.record,
      facets: newFacets,
    },
  };
}

function extractEmotesFromMessage(
  message: ChatMessageViewHydrated,
): EmoteView[] {
  const emotes: EmoteView[] = [];
  if (!message.record.facets) return emotes;

  for (const facet of message.record.facets) {
    for (const feature of facet.features) {
      if (feature.$type === "place.stream.richtext.facet#emote") {
        const emote = feature as {
          name: string;
          ref?: { uri: string; cid: string };
        };
        if (emote.ref) {
          emotes.push({
            aturi: emote.ref.uri,
            cid: emote.ref.cid,
            name: emote.name,
            imageUrl: emoteImageUrl(emote.ref.uri, emote.ref.cid),
          });
        }
      }
    }
  }

  return emotes;
}

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
          badges: message.badges,
        };
        const emotes = extractEmotesFromMessage(hydrated);
        state = reduceChat(state, [hydrated], []);
        if (emotes.length > 0) {
          state = {
            ...state,
            emotes: {
              ...state.emotes,
              ...Object.fromEntries(emotes.map((e) => [e.aturi, e])),
            },
          };
        }
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
      } else if (PlaceStreamChatDefs.isPinnedRecordView(message)) {
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
      } else if (PlaceStreamLiveTeleport.isRecord(message)) {
        const teleportRecord = message as PlaceStreamLiveTeleport.Record;
        state = {
          ...state,
          activeTeleport: teleportRecord,
        };
      } else if (PlaceStreamLivestream.isTeleportArrival(message)) {
        // teleport has succeeded, we are now at the target stream
        const arrival = message as PlaceStreamLivestream.TeleportArrival;

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
      } else if (PlaceStreamLivestream.isTeleportCanceled(message)) {
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
