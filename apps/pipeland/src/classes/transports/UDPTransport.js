
/**
 * UDP implementation of a transport scheme.
 */

import winston from "winston";
import dgram from "dgram";
import url from "url";
import {Readable, Writable} from "stream";

/**
 * Get a random number between min and max. Inclusive of min, exclusive of max.
 * @param  {Number} min
 * @param  {Number} max
 * @return {Number}
 */
const getRandomArbitrary = function(min, max) {
  return Math.floor(Math.random() * (max - min) + min);
};

/**
 * Get a random port.
 * @return {[type]} [description]
 */
const randomPort = function() {
  return getRandomArbitrary(40000, 50000);
};

/**
 * Get something that is hopefully a fresh UDP address.
 */
const getBaseURL = function() {
  return `udp://127.0.0.1:${randomPort()}?`;
};

export function getInputURL() {
  return getBaseURL() + "reuse=1&fifo_size=1000000&buffer_size=1000000&overrun_nonfatal=1";
}

export function getOutputURL() {
  return getBaseURL() + "pkt_size=1880";
}

/**
 * UDPInputStream implements listening on a specified UDP port and piping all data recieved
 * somewhere.
 */
export class InputStream extends Readable {
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
    winston.error("UDP Error", err);
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
export class OutputStream extends Writable {
  constructor(params = {}) {
    super();
    if (params.url) {
      this.setURL(params.url);
    }
    this.sendSocket = dgram.createSocket("udp4");
    this.sendSocket.on("error", this._onError.bind(this));
  }

  setURL(newURL) {
    const {port, hostname} = url.parse(newURL);
    this.port = port;
    this.hostname = hostname;
  }

  _onError(err) {
    winston.error("UDP Error", err);
  }

  _write(chunk, encoding, done) {
    if (this.hostname && this.port) {
      this.sendSocket.send(chunk, 0, chunk.length, this.port, this.hostname);
    }
    done();
  }
}
