
import net from "net";

const PORT = 1730;
const HOST = "drumstick.iame.li";
const REMOTE_PORT = 1934;

const output = new net.Socket({
  readable: true,
  writable: true
});

const server = net.createServer(function(incoming) {
  console.log(`Opening connection to ${HOST}:${REMOTE_PORT}`);
  const outgoing = net.createConnection({
    readable: true,
    writable: true,
    host: HOST,
    port: REMOTE_PORT
  });
  incoming.pipe(outgoing);
  outgoing.pipe(incoming);
});

console.log(`Listening on ${PORT}`);
server.listen(PORT, "127.0.0.1");