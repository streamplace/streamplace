/*eslint-disable no-console */
import WebSocket from "ws";
import {resolve} from "path";

const PORT = process.env.PORT || 80;
const ROOT_DIR = process.env.ROOT_DIR || "/tmp/sp-package-manager";

const wss = new WebSocket.Server({
  perMessageDeflate: false,
  port: PORT
});

wss.on("listening", () => {
  console.log(`⚜  sp-package-manager listening on ${PORT}`);
});

wss.on("connection", (socket) => {
  new FileSyncServer({socket});
});

export class FileSyncServer {
  constructor({socket}) {
    socket.on("message", (data, {binary, masked}) => {
      if (binary) {
        console.log(`Got ${data.length} bytes`);
      }
      else {
        console.log(`Got ${data}`);
      }
    });
  }
}
