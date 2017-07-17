import net from "net";
import os from "os";
import stream from "stream";
import { resolve } from "path";
import debug from "debug";

const log = debug("sp:socket-ingress-stream");

export default function() {
  const tmpDir = os.tmpdir();
  const socketName = `sp-sock-${Date.now()}-${Math.round(
    Math.random() * 1000
  )}.sock`;
  const socketPath = resolve(tmpDir, socketName);

  const server = net.createServer(c => {
    // 'connection' listener
    log("client connected");
    c.on("error", () => {
      log("client errored");
    });
    c.on("end", () => {
      log("client disconnected");
    });
    c.write("hello\r\n");
    c.pipe(socketStream);
  });

  server.on("error", err => {
    socketStream.end();
    throw err;
  });

  server.listen(socketPath, () => {
    log("server bound to " + socketPath);
  });

  const socketStream = new stream.PassThrough();
  socketStream.server = server;
  socketStream.path = socketPath;
  return socketStream;
}
