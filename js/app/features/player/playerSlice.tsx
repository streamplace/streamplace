import { createAction } from "@reduxjs/toolkit";
import { createAppSlice } from "../../hooks/createSlice";
import { uuidv7 } from "hooks/uuid";
import { createContext, useContext } from "react";
import { Repo, StreamplaceState } from "features/streamplace/streamplaceSlice";
import { AppBskyFeedPost } from "@atproto/api";

export interface PlayerContextType {
  playerId: string | null;
}

export const PlayerContext = createContext<PlayerContextType>({
  playerId: null,
});

export interface StreamplaceAppBskyFeedPost extends AppBskyFeedPost.Record {
  "place.stream.livestream": {
    title: string;
    url: string;
  };
}

export interface ChatMessage {
  post: StreamplaceAppBskyFeedPost;
  repo: Repo;
  cid: string;
}

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

interface Segment {
  id: string;
  startTime: number;
  endTime: number;
  mediaData: SegmentMediaData | null;
}

export interface PlayerState {
  ingestStarted: number | null;
  ingestStarting: boolean;
  ingestConnectionState: RTCPeerConnectionState | null;
  viewers: number | null;
  chat: ChatMessage[] | null;
  livestream: StreamplaceAppBskyFeedPost | null;
  segment: Segment | null;
}

export interface PlayersState {
  [key: string]: PlayerState;
}

const initialState: PlayersState = {};

export const newPlayer = createAction("player/newPlayer", function prepare() {
  return {
    payload: { playerId: uuidv7() },
  };
});

const usePlayerId = () => {
  const { playerId } = useContext(PlayerContext);
  if (!playerId) {
    throw new Error("Player context not found");
  }
  return playerId;
};

export const playerSlice = createAppSlice({
  name: "player",
  initialState,

  extraReducers: (builder) => {
    builder.addCase(newPlayer, (state, action) => {
      state[action.payload.playerId] = {
        ingestStarted: null,
        ingestStarting: false,
        ingestConnectionState: null,
        viewers: null,
        chat: null,
        livestream: null,
        segment: null,
      };
    });
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

      pollViewers: create.asyncThunk(
        async (
          { playerId, user }: { playerId: string; user: string },
          { getState },
        ) => {
          const { streamplace } = getState() as {
            streamplace: StreamplaceState;
          };
          const res = await fetch(`${streamplace.url}/api/view-count/${user}`);
          const data = await res.json();
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
          const data = (await res.json()) as ChatMessage[];
          return { playerId, chat: data };
        },
        {
          pending: (state) => {
            // state.status = "loading";
          },
          fulfilled: (state, result) => {
            const previous = state[result.payload.playerId].chat;
            const current = result.payload.chat;
            if (
              previous &&
              current &&
              previous.length > 0 &&
              current.length > 0
            ) {
              const previousLast = previous[previous.length - 1];
              const currentLast = current[current.length - 1];
              const previousFirst = previous[0];
              const currentFirst = current[0];
              if (
                previousLast.cid === currentLast.cid &&
                previousFirst.cid === currentFirst.cid
              ) {
                return state;
              }
            }
            return {
              ...state,
              [result.payload.playerId]: {
                ...state[result.payload.playerId],
                chat: result.payload.chat,
              },
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
          const data = (await res.json()) as StreamplaceAppBskyFeedPost;
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
          const data = (await res.json()) as Segment;
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
    };
  },

  selectors: {
    selectPlayer: (state, playerId: string) => {
      return state[playerId];
    },
    selectChat: (state, playerId: string) => {
      return state[playerId].chat;
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
  };
};

// Action creators are generated for each case reducer function.
export const { selectPlayer, selectChat } = playerSlice.selectors;
export const usePlayer = (): ((state: {
  player: PlayersState;
}) => PlayerState) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId];
};
export const useChat = (): ((state: {
  player: PlayersState;
}) => ChatMessage[] | null) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId].chat;
};
