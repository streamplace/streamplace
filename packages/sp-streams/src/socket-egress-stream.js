import net from "net";
import os from "os";
import stream from "stream";
import { resolve } from "path";
import debug from "debug";

const log = debug("sp:socket-egress-stream");

export default function() {
  const tmpDir = os.tmpdir();
  const socketName = `sp-sock-${Date.now()}-${Math.round(
    Math.random() * 1000
  )}.sock`;
  const socketPath = resolve(tmpDir, socketName);

  const server = net.createServer(c => {
    // 'connection' listener
    log("client connected");
    c.on("end", () => {
      log("client disconnected");
      socketStream.unpipe(c);
    });
    c.on("error", () => {
      log("client errored");
      socketStream.unpipe(c);
    });
    socketStream.pipe(c);
  });

  server.on("error", err => {
    throw err;
  });

  const pathProm = new Promise((resolve, reject) => {
    server.listen(socketPath, () => {
      log("server bound to " + socketPath);
      resolve(socketPath);
    });
  });

  const socketStream = new stream.PassThrough();
  socketStream.getPath = () => pathProm;
  socketStream.server = server;
  socketStream.path = socketPath;
  return socketStream;
}
