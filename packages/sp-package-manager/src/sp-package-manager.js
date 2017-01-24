/*eslint-disable no-console */
import WebSocket from "ws";

const PORT = process.env.PORT || 80;
const ROOT_DIR = process.env.ROOT_DIR || "/tmp/sp-package-manager";

export class FileSyncServer {
  constructor({port, rootDir}) {
    this.wss = new WebSocket.Server({
      perMessageDeflate: false,
      port: PORT
    });

    this.wss.on("listening", () => {
      console.log(`⚜  sp-package-manager listening on ${PORT}`);
    });

    this.wss.on("connection", (socket) => {
      console.log("connection!");

      socket.on("message", (data, {binary, masked}) => {
        if (binary) {
          console.log(`Got ${data.length} bytes`);
        }
        else {
          console.log(`Got ${data}`);
        }
      });
    });
  }
}

new FileSyncServer({port: PORT, rootDir: ROOT_DIR});
