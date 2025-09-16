import React, { createContext, useContext, useMemo } from "react";
import { StoreApi, useStore } from "zustand";
import { PlayerProtocol, PlayerState } from "./player-state";
import { usePlayerContext } from "./player-store";

// Context for a single player
interface SinglePlayerContextType {
  playerId: string;
  playerStore: StoreApi<PlayerState>;
}

const SinglePlayerContext = createContext<SinglePlayerContextType | null>(null);

interface SinglePlayerProviderProps {
  children: React.ReactNode;
  playerId?: string;
  protocol?: PlayerProtocol;
  rendition?: string;
}

/**
 * Provider component for a single player that creates a scoped context
 * This allows components to access a specific player's state without passing IDs around
 */
export const SinglePlayerProvider: React.FC<SinglePlayerProviderProps> = ({
  children,
  playerId: providedPlayerId,
  protocol = PlayerProtocol.WEBRTC,
  rendition = "auto",
}) => {
  const { players, createPlayer } = usePlayerContext();

  // Create or get a player ID
  const playerId = useMemo(() => {
    // If a player ID is provided and exists, use it
    if (providedPlayerId && players[providedPlayerId]) {
      return providedPlayerId;
    }

    // If a player ID is provided but doesn't exist, create it
    if (providedPlayerId) {
      return createPlayer(providedPlayerId);
    }

    // Otherwise create a new player
    return createPlayer();
  }, [providedPlayerId, players, createPlayer]);

  // Get the player store
  const playerStore = useMemo(() => {
    return players[playerId];
  }, [players, playerId]);

  // Set initial protocol and rendition if provided
  React.useEffect(() => {
    if (protocol) {
      playerStore.setState((state) => ({
        ...state,
        protocol,
      }));
    }

    if (rendition) {
      playerStore.setState((state) => ({
        ...state,
        selectedRendition: rendition,
      }));
    }
  }, [playerStore, protocol, rendition]);

  // Create context value
  const contextValue = useMemo(
    () => ({
      playerId,
      playerStore,
    }),
    [playerId, playerStore],
  );

  return (
    <SinglePlayerContext.Provider value={contextValue}>
      {children}
    </SinglePlayerContext.Provider>
  );
};

/**
 * Hook to access the current single player context
 */
export function useSinglePlayerContext() {
  const context = useContext(SinglePlayerContext);
  if (!context) {
    throw new Error(
      "useSinglePlayerContext must be used within a SinglePlayerProvider",
    );
  }
  return context;
}

/**
 * Hook to access the current player ID from the single player context
 */
export function useCurrentPlayerId(): string {
  const { playerId } = useSinglePlayerContext();
  return playerId;
}

/**
 * Hook to access state from the current player without needing to specify the ID
 */
export function useCurrentPlayerStore<U>(
  selector: (state: PlayerState) => U,
): U {
  const { playerStore } = useSinglePlayerContext();
  return useStore(playerStore, selector);
}

/**
 * Hook to get the protocol of the current player
 */
export function useCurrentPlayerProtocol(): [
  PlayerProtocol,
  (protocol: PlayerProtocol) => void,
] {
  return useCurrentPlayerStore(
    (state) => [state.protocol, state.setProtocol] as const,
  );
}

/**
 * Hook to get the selected rendition of the current player
 */
export function useCurrentPlayerRendition(): [
  string,
  (rendition: string) => void,
] {
  return useCurrentPlayerStore(
    (state) => [state.selectedRendition, state.setSelectedRendition] as const,
  );
}

/**
 * Hook to get the ingest state of the current player
 */
export function useCurrentPlayerIngest(): {
  starting: boolean;
  setStarting: (starting: boolean) => void;
  connectionState: RTCPeerConnectionState | null;
  setConnectionState: (state: RTCPeerConnectionState | null) => void;
  startedTimestamp: number | null;
  setStartedTimestamp: (timestamp: number | null) => void;
} {
  return useCurrentPlayerStore((state) => ({
    starting: state.ingestStarting,
    setStarting: state.setIngestStarting,
    connectionState: state.ingestConnectionState,
    setConnectionState: state.setIngestConnectionState,
    startedTimestamp: state.ingestStarted,
    setStartedTimestamp: state.setIngestStarted,
  }));
}

/**
 * HOC to wrap components with a SinglePlayerProvider
 */
export function withSinglePlayer<P extends object>(
  Component: React.ComponentType<P>,
): React.FC<P & SinglePlayerProviderProps> {
  return function WithSinglePlayer(props: P & SinglePlayerProviderProps) {
    const { playerId, protocol, rendition, ...componentProps } = props;
    return (
      <SinglePlayerProvider
        playerId={playerId}
        protocol={protocol}
        rendition={rendition}
      >
        <Component {...(componentProps as P)} />
      </SinglePlayerProvider>
    );
  };
}
