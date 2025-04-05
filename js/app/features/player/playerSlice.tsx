import { AppBskyFeedDefs, AppBskyFeedPost } from "@atproto/api";
import { createAction } from "@reduxjs/toolkit";
import { PROTOCOL_HLS, PROTOCOL_WEBRTC } from "components/player/props";
import { StreamplaceState } from "features/streamplace/streamplaceSlice";
import { uuidv7 } from "hooks/uuid";
import {
  isMessageView,
  MessageView,
} from "lexicons/types/place/stream/chat/defs";
import { createContext, useContext } from "react";
import { createAppSlice } from "../../hooks/createSlice";
import { Record as ChatMessageRecord } from "../../lexicons/types/place/stream/chat/message";
import {
  BlockView,
  isBlockView,
  isRenditions,
  Rendition,
} from "../../lexicons/types/place/stream/defs";
import {
  isLivestreamView,
  isViewerCount,
  Record as LivestreamRecord,
  LivestreamView,
  ViewerCount,
} from "../../lexicons/types/place/stream/livestream";
import * as Segment from "../../lexicons/types/place/stream/segment";
export interface PlayerContextType {
  playerId: string | null;
}

export interface LivestreamViewHydrated extends LivestreamView {
  record: LivestreamRecord;
}
export interface PostViewHydrated extends AppBskyFeedDefs.PostView {
  record: AppBskyFeedPost.Record;
}
export interface MessageViewHydrated extends MessageView {
  record: ChatMessageRecord;
}

export const PlayerContext = createContext<PlayerContextType>({
  playerId: null,
});

interface SegmentMediadataVideo {
  width: number;
  height: number;
  framerate: string;
}

interface SegmentMediadataAudio {
  rate: number;
  channels: number;
}

interface SegmentMediaData {
  video: SegmentMediadataVideo[];
  audio: SegmentMediadataAudio[];
}

export interface PlayerState {
  ingestStarted: number | null;
  ingestStarting: boolean;
  ingestConnectionState: RTCPeerConnectionState | null;
  viewers: number | null;
  chat: { [key: string]: MessageViewHydrated };
  chatList: MessageViewHydrated[];
  livestream: LivestreamViewHydrated | null;
  segment: Segment.Record | null;
  renditions: Rendition[];
  selectedRendition: string | null;
  protocol: string;
}

export interface PlayersState {
  [key: string]: PlayerState;
}

const initialState: PlayersState = {};

export const newPlayer = createAction("player/newPlayer", function prepare() {
  return {
    payload: { playerId: uuidv7(), forceProtocol: PROTOCOL_WEBRTC },
  };
});

const usePlayerId = () => {
  const { playerId } = useContext(PlayerContext);
  if (!playerId) {
    throw new Error("Player context not found");
  }
  return playerId;
};

const reduceChat = (
  state: PlayerState,
  messages: MessageViewHydrated[],
  blocks: BlockView[],
): PlayerState => {
  state = { ...state } as PlayerState;
  const newChat: { [key: string]: MessageViewHydrated } = { ...state.chat };
  for (const message of messages) {
    const date = new Date(message.record.createdAt);
    const key = `${date.getTime()}-${message.uri}`;
    newChat[key] = message;
  }

  for (const block of blocks) {
    for (const [k, v] of Object.entries(newChat)) {
      if (v.author.did === block.record.subject) {
        console.log(
          "deleting message",
          v.cid,
          v.author.did,
          block.record.subject,
        );
        delete newChat[k];
      } else {
        console.log(
          "keeping message",
          v.cid,
          v.author.did,
          block.record.subject,
        );
      }
    }
  }

  const newChatList = Object.keys(newChat)
    .sort((a, b) => (a > b ? 1 : -1))
    .map((key) => newChat[key]);

  return {
    ...state,
    chat: newChat,
    chatList: newChatList,
  };
};

export const playerSlice = createAppSlice({
  name: "player",
  initialState,

  extraReducers: (builder) => {
    builder.addCase(
      newPlayer,
      (
        state,
        action: { payload: { playerId: string; forceProtocol: string } },
      ) => {
        state[action.payload.playerId] = {
          ingestStarted: null,
          ingestStarting: false,
          ingestConnectionState: null,
          viewers: null,
          protocol: action.payload.forceProtocol ?? PROTOCOL_WEBRTC,
          chat: {},
          chatList: [],
          livestream: null,
          segment: null,
          renditions: [],
          selectedRendition: "source",
        };
      },
    );
  },

  reducers: (create) => {
    return {
      startIngest: create.reducer(
        (
          state,
          action: {
            payload: { playerId: string; startIngest: boolean };
            type: string;
          },
        ) => {
          return {
            ...state,
            [action.payload.playerId]: {
              ...state[action.payload.playerId],
              ingestStarting: action.payload.startIngest,
            },
          };
        },
      ),

      ingestConnectionState: create.reducer(
        (
          state,
          action: {
            payload: {
              playerId: string;
              ingestConnectionState: RTCPeerConnectionState;
            };
            type: string;
          },
        ) => {
          return {
            ...state,
            [action.payload.playerId]: {
              ...state[action.payload.playerId],
              ingestConnectionState: action.payload.ingestConnectionState,
            },
          };
        },
      ),

      handleWebSocketMessages: create.reducer(
        (
          state,
          action: {
            payload: { playerId: string; messages: any[] };
            type: string;
          },
        ) => {
          for (const message of action.payload.messages) {
            if (isLivestreamView(message)) {
              state = {
                ...state,
                [action.payload.playerId]: {
                  ...state[action.payload.playerId],
                  livestream: message as LivestreamViewHydrated,
                },
              };
            } else if (isViewerCount(message)) {
              state = {
                ...state,
                [action.payload.playerId]: {
                  ...state[action.payload.playerId],
                  viewers: message.count,
                },
              };
            } else if (isMessageView(message)) {
              state = {
                ...state,
                [action.payload.playerId]: reduceChat(
                  state[action.payload.playerId] as PlayerState,
                  [message as MessageViewHydrated],
                  [],
                ),
              };
            } else if (Segment.isRecord(message)) {
              state = {
                ...state,
                [action.payload.playerId]: {
                  ...state[action.payload.playerId],
                  segment: message as Segment.Record,
                },
              };
            } else if (isBlockView(message)) {
              const block = message as BlockView;
              state = {
                ...state,
                [action.payload.playerId]: reduceChat(
                  state[action.payload.playerId] as PlayerState,
                  [],
                  [block],
                ),
              };
            } else if (isRenditions(message)) {
              state = {
                ...state,
                [action.payload.playerId]: {
                  ...state[action.payload.playerId],
                  renditions: message.renditions,
                },
              };
            }
          }
          return state;
        },
      ),

      pollViewers: create.asyncThunk(
        async (
          { playerId, user }: { playerId: string; user: string },
          { getState },
        ) => {
          const { streamplace } = getState() as {
            streamplace: StreamplaceState;
          };
          const res = await fetch(`${streamplace.url}/api/view-count/${user}`);
          const data = (await res.json()) as ViewerCount;
          return { playerId, count: data.count };
        },
        {
          pending: (state) => {
            // state.status = "loading";
          },
          fulfilled: (state, result) => {
            return {
              ...state,
              [result.payload.playerId]: {
                ...state[result.payload.playerId],
                viewers: result.payload.count,
              },
            };
          },
          rejected: (state, error) => {
            console.error("pollViewers rejected", error);
            return state;
          },
        },
      ),

      pollChat: create.asyncThunk(
        async (
          { playerId, user }: { playerId: string; user: string },
          { getState },
        ) => {
          const { streamplace } = getState() as {
            streamplace: StreamplaceState;
          };
          const res = await fetch(`${streamplace.url}/api/chat/${user}`);
          const data = (await res.json()) as MessageViewHydrated[];
          return { playerId, chat: data };
        },
        {
          pending: (state) => {
            // state.status = "loading";
          },
          fulfilled: (state, result) => {
            return {
              ...state,
              [result.payload.playerId]: reduceChat(
                state[result.payload.playerId] as PlayerState,
                result.payload.chat,
                [],
              ),
            };
          },
          rejected: (state, error) => {
            console.error("pollViewers rejected", error);
            return state;
          },
        },
      ),

      pollLivestream: create.asyncThunk(
        async (
          { playerId, user }: { playerId: string; user: string },
          { getState },
        ) => {
          const { streamplace } = getState() as {
            streamplace: StreamplaceState;
          };
          const res = await fetch(`${streamplace.url}/api/livestream/${user}`);
          const data = (await res.json()) as LivestreamViewHydrated;
          return { playerId, livestream: data };
        },
        {
          pending: (state) => {
            // state.status = "loading";
          },
          fulfilled: (state, result) => {
            return {
              ...state,
              [result.payload.playerId]: {
                ...state[result.payload.playerId],
                livestream: result.payload.livestream,
              },
            };
          },
          rejected: (state, error) => {
            console.error("pollViewers rejected", error);
            return state;
          },
        },
      ),

      pollSegment: create.asyncThunk(
        async (
          { playerId, user }: { playerId: string; user: string },
          { getState },
        ) => {
          const { streamplace } = getState() as {
            streamplace: StreamplaceState;
          };
          const res = await fetch(
            `${streamplace.url}/api/segment/recent/${user}`,
          );
          const data = (await res.json()) as Segment.Record;
          return { playerId, segment: data };
        },
        {
          pending: (state) => {
            // state.status = "loading";
          },
          fulfilled: (state, result) => {
            return {
              ...state,
              [result.payload.playerId]: {
                ...state[result.payload.playerId],
                segment: result.payload.segment,
              },
            };
          },
          rejected: (state, error) => {
            console.error("pollViewers rejected", error);
            return state;
          },
        },
      ),

      setSelectedRendition: create.reducer(
        (
          state,
          action: {
            payload: { playerId: string; rendition: string };
            type: string;
          },
        ) => {
          return {
            ...state,
            [action.payload.playerId]: {
              ...state[action.payload.playerId],
              selectedRendition: action.payload.rendition,
            },
          };
        },
      ),

      setProtocol: create.reducer(
        (
          state,
          action: {
            payload: { playerId: string; protocol: string };
            type: string;
          },
        ) => {
          const newPlayer = {
            ...state[action.payload.playerId],
            protocol: action.payload.protocol,
          };
          if (action.payload.protocol === PROTOCOL_HLS) {
            newPlayer.selectedRendition = "auto";
          } else {
            if (newPlayer.selectedRendition === "auto") {
              newPlayer.selectedRendition = "source";
            }
          }
          return {
            ...state,
            [action.payload.playerId]: newPlayer,
          };
        },
      ),
    };
  },

  selectors: {
    selectPlayer: (state, playerId: string) => {
      return state[playerId];
    },
    selectChat: (state, playerId: string) => {
      return state[playerId].chat;
    },
    selectLivestream: (state, playerId: string) => {
      return state[playerId].livestream;
    },
    selectSegment: (state, playerId: string) => {
      return state[playerId].segment;
    },
    selectRenditions: (state, playerId: string) => {
      return state[playerId].renditions;
    },
    selectSelectedRendition: (state, playerId: string) => {
      return state[playerId].selectedRendition;
    },
    selectProtocol: (state, playerId: string) => {
      return state[playerId].protocol;
    },
  },
});

export const usePlayerActions = () => {
  const playerId = usePlayerId();
  return {
    startIngest: (startIngest: boolean) =>
      playerSlice.actions.startIngest({ playerId, startIngest }),
    ingestConnectionState: (ingestConnectionState: RTCPeerConnectionState) => {
      console.log("ingestConnectionState", ingestConnectionState);
      return playerSlice.actions.ingestConnectionState({
        playerId,
        ingestConnectionState,
      });
    },
    pollViewers: (user: string) =>
      playerSlice.actions.pollViewers({ playerId, user }),
    pollChat: (user: string) =>
      playerSlice.actions.pollChat({ playerId, user }),
    pollLivestream: (user: string) =>
      playerSlice.actions.pollLivestream({ playerId, user }),
    pollSegment: (user: string) =>
      playerSlice.actions.pollSegment({ playerId, user }),
    handleWebSocketMessages: (messages: any[]) =>
      playerSlice.actions.handleWebSocketMessages({ playerId, messages }),
    setSelectedRendition: (rendition: string) =>
      playerSlice.actions.setSelectedRendition({ playerId, rendition }),
    setProtocol: (protocol: string) =>
      playerSlice.actions.setProtocol({ playerId, protocol }),
  };
};

// Action creators are generated for each case reducer function.
export const { selectPlayer, selectChat, selectLivestream, selectSegment } =
  playerSlice.selectors;
export const usePlayer = (): ((state: {
  player: PlayersState;
}) => PlayerState) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId];
};
export const useChat = (): ((state: {
  player: PlayersState;
}) => MessageViewHydrated[] | null) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId].chatList;
};
export const usePlayerLivestream = (): ((state: {
  player: PlayersState;
}) => LivestreamViewHydrated | null) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId].livestream;
};
export const usePlayerSegment = (): ((state: {
  player: PlayersState;
}) => Segment.Record | null) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId].segment;
};
export const usePlayerRenditions = (): ((state: {
  player: PlayersState;
}) => Rendition[]) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId].renditions;
};
export const usePlayerSelectedRendition = (): ((state: {
  player: PlayersState;
}) => string | null) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId].selectedRendition;
};
export const usePlayerProtocol = (): ((state: {
  player: PlayersState;
}) => string) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId].protocol;
};
