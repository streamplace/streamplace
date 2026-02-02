import { useRef } from "react";
import useWebSocket from "react-use-websocket";
import { useHandleWebsocketMessages } from "../livestream-store";
import { useDID, useStreamplaceStore, useUrl } from "../streamplace-store";

export function useLivestreamWebsocket(src: string) {
  const url = useUrl();
  const did = useDID();
  const oauthSession = useStreamplaceStore((state) => state.oauthSession);
  const handleWebSocketMessages = useHandleWebsocketMessages();

  let wsUrl = url.replace(/^http\:/, "ws:");
  wsUrl = wsUrl.replace(/^https\:/, "wss:");

  const ref = useRef<any[]>([]);
  const handle = useRef<NodeJS.Timeout | null>(null);
  const hasReceivedMessage = useRef(false);
  const hasErrored = useRef(false);

  // Don't connect until auth state is resolved (undefined = still loading)
  const authResolved = oauthSession !== undefined;

  const wsUrlWithViewer = did
    ? `${wsUrl}/api/websocket/${src}?viewer=${encodeURIComponent(did)}`
    : `${wsUrl}/api/websocket/${src}`;

  const { readyState } = useWebSocket(authResolved ? wsUrlWithViewer : null, {
    reconnectInterval: 1000,
    shouldReconnect: () => !hasErrored.current,

    onOpen: () => {
      ref.current = [];
      hasReceivedMessage.current = false;
    },

    onError: (e) => {
      console.log("onError", e);
      if (!hasReceivedMessage.current) {
        hasErrored.current = true;
        handleWebSocketMessages([
          {
            $type: "place.stream.error",
            code: "user_not_found",
            message: "this stream doesn't exist or is unavailable",
          },
        ]);
      }
    },

    // spamming the redux store with messages causes a zillion re-renders,
    // so we batch them up a bit
    onMessage: (msg) => {
      try {
        const data = JSON.parse(msg.data);
        hasReceivedMessage.current = true;
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
