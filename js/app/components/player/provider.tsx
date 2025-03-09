// basically PlayerProvider that sets up our magic context,

import { newPlayer, PlayerContext } from "features/player/playerSlice";
import { useState, useEffect, useContext } from "react";
import { useAppDispatch } from "store/hooks";
import { PlayerProps } from "./props";

// PlayerInner starts doing player stuff
export default function PlayerProvider(
  props: Partial<PlayerProps> & { children: React.ReactNode },
): React.ReactNode {
  const ctx = useContext(PlayerContext);
  if (ctx.playerId) {
    return props.children;
  }
  return <PlayerContextInitializer>{props.children}</PlayerContextInitializer>;
}

export function PlayerContextInitializer(
  props: Partial<PlayerProps> & { children: React.ReactNode },
) {
  const dispatch = useAppDispatch();
  const [playerId, setPlayerId] = useState<string | null>(null);
  useEffect(() => {
    const newPlayerAction = newPlayer();
    if (props.playerId) {
      newPlayerAction.payload.playerId = props.playerId;
    }
    setPlayerId(newPlayerAction.payload.playerId);
    dispatch(newPlayerAction);
  }, []);
  if (!playerId) {
    return <></>;
  }
  return (
    <PlayerContext.Provider value={{ playerId }}>
      {props.children}
    </PlayerContext.Provider>
  );
}
