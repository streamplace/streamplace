import { AppBskyFeedDefs, AppBskyFeedPost } from "@atproto/api";
import { createAction } from "@reduxjs/toolkit";
import { PROTOCOL_HLS, PROTOCOL_WEBRTC } from "components/player/props";
import { uuidv7 } from "hooks/uuid";
import { createContext, useContext } from "react";
import { useAppDispatch } from "store/hooks";
import { PlaceStreamChatMessage, PlaceStreamLivestream } from "streamplace";
import { createAppSlice } from "../../hooks/createSlice";

export interface PlayerContextType {
  playerId: string | null;
}

export interface LivestreamViewHydrated
  extends PlaceStreamLivestream.LivestreamView {
  record: PlaceStreamLivestream.Record;
}
export interface PostViewHydrated extends AppBskyFeedDefs.PostView {
  record: AppBskyFeedPost.Record;
}
export interface MessageViewHydrated {
  uri: string;
  cid: string;
  author: Author;
  record: PlaceStreamChatMessage.Record;
  indexedAt: string;
  chatProfile?: ChatProfile;
}

export const PlayerContext = createContext<PlayerContextType>({
  playerId: null,
});

export interface PlayerState {
  ingestStarted: number | null;
  ingestStarting: boolean;
  ingestConnectionState: RTCPeerConnectionState | null;
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

interface ChatProfileColor {
  red: number;
  green: number;
  blue: number;
}

interface ChatProfile {
  color?: ChatProfileColor;
}

interface Author {
  did: string;
  handle: string;
}

export const usePlayerId = () => {
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
          protocol: action.payload.forceProtocol ?? PROTOCOL_WEBRTC,
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
  const dispatch = useAppDispatch();
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
    setSelectedRendition: (rendition: string) =>
      playerSlice.actions.setSelectedRendition({ playerId, rendition }),
    setProtocol: (protocol: string) =>
      playerSlice.actions.setProtocol({ playerId, protocol }),
  };
};

// Action creators are generated for each case reducer function.
export const { selectPlayer } = playerSlice.selectors;
export const usePlayer = (): ((state: {
  player: PlayersState;
}) => PlayerState) => {
  const playerId = usePlayerId();
  return (state) => state.player[playerId];
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
