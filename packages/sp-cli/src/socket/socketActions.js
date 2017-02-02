
import WebSocket from "ws";
import {
  SOCKET_LISTEN,
  SOCKET_LISTEN_SUCCESS,
  SOCKET_CONNECT,
  SOCKET_CONNECT_SUCCESS,
  SOCKET_CONNECT_ERROR,
  SOCKET_CONNECT_TIMEOUT,
  SOCKET_CLOSE,
  SOCKET_ERROR,
  SOCKET_MESSAGE,
  SOCKET_PING,
  SOCKET_PONG,
  SOCKET_UNEXPECTED_RESPONSE,
} from "../constants/actionNames";

const CONNECT_TIMEOUT = 3000;
const CONNECT_RETRY = 3000;

export const socketListen = () => (dispatch, getState) => {
  const port = getState().socket.port;
  dispatch({
    type: SOCKET_LISTEN,
    port: port,
  });

  const wss = new WebSocket.Server({port});
  wss.on("listening", () => {
    dispatch({
      type: SOCKET_LISTEN_SUCCESS,
      port: port,
    });
  });
};

export const socketConnect = () => (dispatch, getState) => {
  const server = getState().socket.devServer;
  dispatch({
    type: SOCKET_CONNECT,
    server: server,
  });

  const ws = new WebSocket(server);

  const timeout = setTimeout(() => {
    ws.close();
    dispatch({
      type: SOCKET_CONNECT_TIMEOUT,
      server: server,
    });
    setTimeout(() => {
      socketConnect()(dispatch, getState);
    }, CONNECT_RETRY);
  }, CONNECT_TIMEOUT);

  ws.on("open", () => {
    clearTimeout(timeout);
    dispatch({
      type: SOCKET_CONNECT_SUCCESS,
      server: server,
    });
  });

  ws.on("close", (code, reason) => {
    clearTimeout(timeout);
    dispatch({
      type: SOCKET_CLOSE,
      server: server,
      reason: reason,
    });
    setTimeout(() => {
      socketConnect()(dispatch, getState);
    }, CONNECT_RETRY);
  });

  ws.on("error", (err) => {
    dispatch({
      type: SOCKET_ERROR,
      server: server,
      error: err,
    });
  });

  ws.on("message", (message) => {
    dispatch({
      type: SOCKET_MESSAGE,
      message: message,
      server: server,
    });
  });

  ws.on("unexpected-response", (request, response) => {
    dispatch({
      type: SOCKET_UNEXPECTED_RESPONSE,
      request: request,
      response: response,
    });
  });
};
