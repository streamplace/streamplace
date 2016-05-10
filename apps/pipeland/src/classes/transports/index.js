
import url from "url";

import * as udp from "./UDPTransport";
import * as tcp from "./TCPTransport";

const protocols = {
  "udp": udp,
  "tcp": tcp,
};

export function getTransport(protocol) {
  // When we get out from url.parse we have a trailing colon. Account for that here.
  if (protocol[protocol.length - 1] === ":") {
    protocol = protocol.slice(0, protocol.length - 1);
  }
  if (protocols[protocol]) {
    return protocols[protocol];
  }
  throw new Error(`Unknown protocol: ${protocol}`);
}

export function getTransportFromURL(inputURL) {
  const {protocol} = url.parse(inputURL);
  return getTransport(protocol);
}
