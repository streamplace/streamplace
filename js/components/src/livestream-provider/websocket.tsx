import { useRef } from "react";
import useWebSocket from "react-use-websocket";
import {
  getStoreFromContext,
  useHandleWebsocketMessages,
} from "../livestream-store/use-store";
import { useUrl } from "../streamplace-store";

export function useLivestreamWebsocket(src: string) {
  const url = useUrl();
  const handleWebSocketMessages = useHandleWebsocketMessages();
  const store = getStoreFromContext();

  let wsUrl = url.replace(/^http\:/, "ws:");
  wsUrl = wsUrl.replace(/^https\:/, "wss:");

  const ref = useRef<any[]>([]);
  const handle = useRef<NodeJS.Timeout | null>(null);

  useWebSocket(`${wsUrl}/api/websocket/${src}`, {
    reconnectInterval: 1000,
    shouldReconnect: () => true,

    onOpen: () => {
      ref.current = [];
    },

    // websocketConnected turns on with the first message, so a socket that
    // goes away has to turn it back off — otherwise viewers (and the overlay
    // status card) keep claiming a connection that is being retried. Core's
    // connect() does the same on close/error.
    onClose: () => {
      store.setState((s) => ({ ...s, websocketConnected: false }));
    },

    onError: (e) => {
      store.setState((s) => ({ ...s, websocketConnected: false }));
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
