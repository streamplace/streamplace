// basically PlayerProvider that sets up our magic context,

import {
  newPlayer,
  PlayerContext,
  usePlayerActions,
} from "features/player/playerSlice";
import { selectUrl } from "features/streamplace/streamplaceSlice";
import { useContext, useEffect, useRef, useState } from "react";
import useWebSocket, { ReadyState } from "react-use-websocket";
import { useAppDispatch, useAppSelector } from "store/hooks";
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
    handleWebSocketMessages,
  } = usePlayerActions();

  const url = useAppSelector(selectUrl);
  let wsUrl = url.replace(/^http\:/, "ws:");
  wsUrl = wsUrl.replace(/^https\:/, "wss:");

  const ref = useRef<any[]>([]);
  const handle = useRef<NodeJS.Timeout | null>(null);

  const { readyState } = useWebSocket(`${wsUrl}/api/websocket/${props.src}`, {
    reconnectInterval: 1000,
    shouldReconnect: () => true,

    onOpen: () => {
      ref.current = [];
    },

    onError: (e) => {
      console.log("onError", e);
    },

    // spamming the redux store with messages causes a zillion re-renders,
    // so we batch them up a bit
    onMessage: (msg) => {
      try {
        const data = JSON.parse(msg.data);
        ref.current.push(data);
        if (handle.current) {
          return;
        }
        handle.current = setTimeout(() => {
          dispatch(handleWebSocketMessages(ref.current));
          ref.current = [];
          handle.current = null;
        }, 250);
      } catch (e) {
        console.log("onMessage parse error", e);
      }
    },
  });

  useEffect(() => {
    return () => {
      if (handle.current) {
        clearTimeout(handle.current);
        handle.current = null;
      }
    };
  }, []);

  useEffect(() => {
    console.log(`websocket ${readyStateNames[readyState]}`);
  }, [readyState]);

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
