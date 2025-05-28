import { useRef } from "react";
import useWebSocket from "react-use-websocket";
import { useHandleWebsocketMessages } from "../livestream-store";
import { useUrl } from "../streamplace-store";

export function useLivestreamWebsocket(src: string) {
  const url = useUrl();
  const handleWebSocketMessages = useHandleWebsocketMessages();

  let wsUrl = url.replace(/^http\:/, "ws:");
  wsUrl = wsUrl.replace(/^https\:/, "wss:");

  const ref = useRef<any[]>([]);
  const handle = useRef<NodeJS.Timeout | null>(null);

  const { readyState } = useWebSocket(`${wsUrl}/api/websocket/${src}`, {
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
          handleWebSocketMessages(ref.current);
          ref.current = [];
          handle.current = null;
        }, 250);
      } catch (e) {
        console.log("onMessage parse error", e);
      }
    },
  });
}
