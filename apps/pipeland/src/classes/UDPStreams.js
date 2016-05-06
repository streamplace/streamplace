
import dgram from "dgram";
import url from "url";
import {Readable, Writable} from "stream";

/**
 * UDPInputStream implements listening on a specified UDP port and piping all data recieved
 * somewhere.
 */
export class UDPInputStream extends Readable {
  constructor(params = {}) {
    super();
    const {port, hostname} = url.parse(params.url);
    this.port = port;
    this.hostname = hostname;
  }

  _read() {
    if (!this.server) {
      this.server = dgram.createSocket("udp4");
      this.server.on("message", this._onMessage.bind(this));
      this.server.on("error", this._onError.bind(this));
      this.server.bind(this.port);
    }
  }

  _onMessage(chunk, rdata) {
    const result = this.push(chunk);
    if (result === false) {
      this.server.close();
      delete this.server;
    }
  }

  _onError(err) {
    throw err;
  }

  stop() {
    if (this.server) {
      this.server.close();
      this.push(null);
    }
  }
}

/**
 * UDPOutputStream implements piping all data recieved to somewhere on the internet over UDP.
 */
export class UDPOutputStream extends Writable {
  constructor(params = {}) {
    super();
    if (params.url) {
      this.setURL(params.url);
    }
    this.sendSocket = dgram.createSocket("udp4");
  }

  setURL(newURL) {
    const {port, hostname} = url.parse(newURL);
    this.port = port;
    this.hostname = hostname;
  }

  _write(chunk, encoding, done) {
    if (this.hostname && this.port) {
      this.sendSocket.send(chunk, 0, chunk.length, this.port, this.hostname);
    }
    done();
  }
}
