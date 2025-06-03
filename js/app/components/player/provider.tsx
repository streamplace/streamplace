// basically PlayerProvider that sets up our magic context,

import { LivestreamProvider } from "@streamplace/components";
import { newPlayer, PlayerContext } from "features/player/playerSlice";
import { useContext, useEffect, useState } from "react";
import { ReadyState } from "react-use-websocket";
import { useAppDispatch } from "store/hooks";
import { PlayerProps } from "./props";

const POLL_INTERVAL = 3000;
// PlayerInner starts doing player stuff
export default function PlayerProvider(
  props: Partial<PlayerProps> & { children: React.ReactNode },
): React.ReactNode {
  const ctx = useContext(PlayerContext);
  if (ctx.playerId) {
    return props.children;
  }
  return (
    <LivestreamProvider src={props.src ?? ""}>
      <PlayerContextInitializer {...props}>
        {props.children}
      </PlayerContextInitializer>
    </LivestreamProvider>
  );
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
    if (props.forceProtocol) {
      newPlayerAction.payload.forceProtocol = props.forceProtocol;
    }
    dispatch(newPlayerAction);
    setPlayerId(newPlayerAction.payload.playerId);
    // if needed, prop back up
    props.setPlayerId?.(newPlayerAction.payload.playerId);
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

const readyStateNames = {
  [ReadyState.CLOSED]: "CLOSED",
  [ReadyState.OPEN]: "OPEN",
  [ReadyState.CONNECTING]: "CONNECTING",
  [ReadyState.CLOSING]: "CLOSING",
  [ReadyState.UNINSTANTIATED]: "UNINSTANTIATED",
};
