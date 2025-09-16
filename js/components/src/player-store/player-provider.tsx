import React, { useCallback, useMemo, useState } from "react";
import { StoreApi } from "zustand";
import { PlayerContext } from "./context";
import { PlayerState } from "./player-state";
import { makePlayerStore } from "./player-store";

interface PlayerProviderProps {
  children: React.ReactNode;
  initialPlayers?: string[];
  defaultId?: string;
}

function randomUUID(): string {
  let dt = new Date().getTime();
  var uuid = "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(
    /[xy]/g,
    function (c) {
      var r = (dt + Math.random() * 16) % 16 | 0;
      dt = Math.floor(dt / 16);
      return (c == "x" ? r : (r & 0x3) | 0x8).toString(16);
    },
  );
  return uuid;
}

export const PlayerProvider: React.FC<PlayerProviderProps> = ({
  children,
  initialPlayers = [],
  defaultId = Math.random().toString(36).slice(8),
}) => {
  const [players, setPlayers] = useState<Record<string, StoreApi<PlayerState>>>(
    () => {
      // Initialize with any initial player IDs provided
      const initialPlayerStores: Record<string, StoreApi<PlayerState>> = {};
      for (const playerId of initialPlayers) {
        initialPlayerStores[playerId] = makePlayerStore(playerId);
      }

      // Always create at least one player by default
      if (initialPlayers.length === 0) {
        initialPlayerStores[defaultId] = makePlayerStore(defaultId);
      }

      return initialPlayerStores;
    },
  );

  const createPlayer = useCallback((id?: string) => {
    const playerId = id || randomUUID();
    const playerStore = makePlayerStore(playerId);

    setPlayers((prev) => ({
      ...prev,
      [playerId]: playerStore,
    }));

    return playerId;
  }, []);

  const removePlayer = useCallback((id: string) => {
    setPlayers((prev) => {
      // Don't remove the last player
      if (Object.keys(prev).length <= 1) {
        console.warn("Cannot remove the last player");
        return prev;
      }

      const newPlayers = { ...prev };
      delete newPlayers[id];
      return newPlayers;
    });
  }, []);

  const contextValue = useMemo(
    () => ({
      players,
      createPlayer,
      removePlayer,
    }),
    [players, createPlayer, removePlayer],
  );

  return (
    <PlayerContext.Provider value={contextValue}>
      {children}
    </PlayerContext.Provider>
  );
};

// HOC to wrap components that need player context
export function withPlayerProvider<P extends object>(
  Component: React.ComponentType<P>,
): React.FC<P & { initialPlayers?: string[] }> {
  return function WithPlayerProvider(props: P & { initialPlayers?: string[] }) {
    const { initialPlayers, ...componentProps } = props;
    return (
      <PlayerProvider initialPlayers={initialPlayers}>
        <Component {...(componentProps as P)} />
      </PlayerProvider>
    );
  };
}
