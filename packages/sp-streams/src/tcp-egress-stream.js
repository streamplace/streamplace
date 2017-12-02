import net from "net";
import stream from "stream";
import { resolve } from "path";
import debug from "debug";

// this stream should "auto-resume". don't send it data if you want backpressure

const log = debug("sp:tcp-egress-stream");

export default function({ port } = {}) {
  let clients = [];

  const server = net.createServer(c => {
    clients.push(c);
    // 'connection' listener
    log("client connected");
    c.on("error", () => {
      log("client errored");
      clients = clients.filter(client => client !== c);
    });
    c.on("end", () => {
      log("client disconnected");
      clients = clients.filter(client => client !== c);
    });
  });

  server.on("error", err => {
    throw err;
  });

  const portProm = new Promise((resolve, reject) => {
    let listen;
    if (port) {
      listen = server.listen.bind(server, port);
    } else {
      listen = server.listen.bind(server);
    }
    listen(() => {
      const port = server.address().port;
      log(`tcp server listening on port ${port}`);
      resolve(port);
    });
  });

  const passThrough = new stream.PassThrough();
  passThrough.server = server;
  passThrough.getPort = () => portProm;
  passThrough.on("data", chunk => {
    clients.forEach(c => c.write(chunk));
  });
  passThrough.on("end", () => {
    server.close();
  });
  return passThrough;
}
