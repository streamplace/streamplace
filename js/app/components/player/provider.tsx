// basically PlayerProvider that sets up our magic context,

import {
  newPlayer,
  PlayerContext,
  usePlayerActions,
} from "features/player/playerSlice";
import { useState, useEffect, useContext } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { PlayerProps } from "./props";
import { selectUrl } from "features/streamplace/streamplaceSlice";
import useWebSocket from "react-use-websocket";
import { ReadyState } from "react-use-websocket";

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
    <PlayerContextInitializer {...props}>
      {props.children}
    </PlayerContextInitializer>
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
    setPlayerId(newPlayerAction.payload.playerId);
    dispatch(newPlayerAction);
  }, []);
  if (!playerId) {
    return <></>;
  }
  return (
    <PlayerContext.Provider value={{ playerId }}>
      <PlayerDataContext {...props} />
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

export function PlayerDataContext(
  props: Partial<PlayerProps> & { children: React.ReactNode },
) {
  const dispatch = useAppDispatch();
  const {
    pollViewers,
    pollChat,
    pollLivestream,
    pollSegment,
    handleWebSocketMessage,
  } = usePlayerActions();

  const url = useAppSelector(selectUrl);
  let wsUrl = url.replace(/^http\:/, "ws:");
  wsUrl = wsUrl.replace(/^https\:/, "wss:");

  const { lastJsonMessage, readyState } = useWebSocket(
    `${wsUrl}/api/websocket/${props.src}`,
    {
      reconnectInterval: 1000,
      shouldReconnect: () => true,
    },
  );

  useEffect(() => {
    console.log(`websocket ${readyStateNames[readyState]}`);
  }, [readyState]);

  useEffect(() => {
    if (!lastJsonMessage) {
      return;
    }
    dispatch(handleWebSocketMessage(lastJsonMessage));
  }, [lastJsonMessage]);

  useEffect(() => {
    if (readyState === ReadyState.OPEN || !props.src) {
      return;
    }
    let handle;
    const poll = async () => {
      if (!props.src) {
        return;
      }
      await Promise.all([
        dispatch(pollViewers(props.src)),
        dispatch(pollChat(props.src)),
        dispatch(pollLivestream(props.src)),
        dispatch(pollSegment(props.src)),
      ]);
    };
    handle = setInterval(poll, POLL_INTERVAL);
    return () => clearInterval(handle);
  }, [props.src, readyState === ReadyState.OPEN]);

  return props.children;
}
