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

  // Much of this logic is dedicated to combating ffmpeg's socket behavior... it makes one
  // "probe" connection, expects data, then connects again for the primary data feed. We don't
  // want to discard the data in the probe, so we don't.
  let first = true;
  let firstBuffer = [];
  let firstBufferLength = 0;
  let firstListener;
  const stopFirstListener = () => {
    if (!firstListener) {
      return;
    }
    log(`First buffer size: ${firstBufferLength}`);
    socketStream.removeListener("data", firstListener);
    firstListener = null;
  };
  const server = net.createServer(c => {
    if (first) {
      first = false;
      firstListener = chunk => {
        c.write(chunk);
        firstBufferLength += chunk.length;
        firstBuffer.push(chunk);
      };
      socketStream.on("data", firstListener);
    } else {
      firstBuffer.forEach(chunk => {
        c.write(chunk);
      });
      firstBuffer = null;
      socketStream.pipe(c);
    }

    log("client connected");
    c.on("end", () => {
      log("client disconnected");
      socketStream.unpipe(c);
      stopFirstListener();
    });
    c.on("error", () => {
      log("client errored");
      socketStream.unpipe(c);
      stopFirstListener();
    });
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
