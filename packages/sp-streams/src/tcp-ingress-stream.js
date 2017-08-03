import net from "net";
import stream from "stream";
import { resolve } from "path";
import debug from "debug";
import { parse as urlParse } from "url";
import mpegMungerStream from "./mpeg-munger-stream";

const log = debug("sp:tcp-ingress-stream");

export default function({ url }) {
  const { protocol, hostname, port } = urlParse(url);
  if (protocol !== "tcp:") {
    throw new Error(`Unexpected protocol for tcpIngressStream: ${protocol}`);
  }

  const tcpIngress = net.createConnection({
    host: hostname,
    port: port
  });

  tcpIngress.on("error", () => {
    debug(`tcpIngress for ${hostname}:${port} errored`);
  });

  const mpegMunger = mpegMungerStream();
  tcpIngress.pipe(mpegMunger);

  // let clients = [];

  // const server = net.createServer(c => {
  //   clients.push(c);
  //   // 'connection' listener
  //   log("client connected");
  //   c.on("end", () => {
  //     log("client disconnected");
  //     clients = clients.filter(client => client !== c);
  //   });
  // });

  // server.on("error", err => {
  //   throw err;
  // });

  // const portProm = new Promise((resolve, reject) => {
  //   server.listen(() => {
  //     const port = server.address().port;
  //     log(`tcp server listening on port ${port}`);
  //     resolve(port);
  //   });
  // });

  return mpegMunger;
}
