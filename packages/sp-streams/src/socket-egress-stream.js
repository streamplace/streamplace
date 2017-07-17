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
    c.write("hello\r\n");
    socketStream.pipe(c);
  });

  server.on("error", err => {
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
