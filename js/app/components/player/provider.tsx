// basically PlayerProvider that sets up our magic context,

import {
  newPlayer,
  PlayerContext,
  usePlayerActions,
} from "features/player/playerSlice";
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

  useEffect(() => {
    if (!props.src) {
      return;
    }

    // Create WebSocket connection
    const wsUrl = `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/api/websocket/${props.src}`;
    const socket = new WebSocket(wsUrl);

    socket.onopen = () => {
      console.log(`WebSocket connection established for ${props.src}`);
    };

    socket.onerror = (error) => {
      console.error(`WebSocket error for ${props.src}:`, error);
    };

    socket.onmessage = (event) => {
      let data: any;
      try {
        data = JSON.parse(event.data);
      } catch (error) {
        console.error(`WebSocket message for ${props.src}:`, error);
        return;
      }
      dispatch(handleWebSocketMessage(data));
    };

    // Clean up the WebSocket connection when component unmounts or src changes
    return () => {
      if (
        socket.readyState === WebSocket.OPEN ||
        socket.readyState === WebSocket.CONNECTING
      ) {
        console.log(`Closing WebSocket connection for ${props.src}`);
        socket.close();
      }
    };
  }, [props.src]);

  useEffect(() => {
    if (!props.src) {
      return;
    }
    const poll = ((src) => async () => {
      await Promise.all([
        // dispatch(pollViewers(src)),
        // dispatch(pollChat(src)),
        dispatch(pollLivestream(src)),
        dispatch(pollSegment(src)),
      ]);
    })(props.src);
    poll();
    const handle = setInterval(poll, 3000);
    return () => clearInterval(handle);
  }, [props.src]);

  return props.children;
}
