import WebSocket from "ws";
import {
  SERVER_LISTEN,
  SERVER_LISTEN_SUCCESS,
  SERVER_LISTEN_ERROR,
  SOCKET_CONNECT_TIMEOUT
} from "../constants/actionNames";

export const serverListen = () => (dispatch, getState) => {
  const port = getState().socket.port;
  dispatch({
    type: SERVER_LISTEN,
    port: port
  });

  const wss = new WebSocket.Server({ port });
  wss.on("listening", () => {
    dispatch({
      type: SERVER_LISTEN_SUCCESS,
      port: port
    });
  });

  wss.on("connection", socket => {
    socket.on("message", (data, { binary, masked }) => {
      if (binary) {
        // console.log(`Got ${data.length} bytes`);
      } else {
        // console.log(`Got ${data}`);
      }
    });
  });
};
