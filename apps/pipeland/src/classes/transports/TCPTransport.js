
/**
 * UDP implementation of a transport scheme.
 */

import winston from "winston";
import net from "net";
import url from "url";
import {Readable, Writable} from "stream";

import {randomPort} from "./utils";
import {MY_IP} from "../../constants";

/**
 * Get something that is hopefully a fresh UDP address.
 */
const getBaseURL = function() {
  return `tcp://${MY_IP}:${randomPort()}`;
};

export function getInputURL() {
  return `${getBaseURL()}?listen`;
}

export function getOutputURL() {
  return `${getBaseURL()}?recv_buffer_size=1000000`;
}

let listens = [];

/**
 * InputStream implements listening on a specified TCP port and piping all data recieved
 * somewhere.
 */
export class InputStream extends Readable {
  constructor(params = {}) {
    super();
    const {port, hostname} = url.parse(params.url);
    this.port = port;
    this.hostname = hostname;
    this.pushing = false;
    this.server = net.createServer();
    this.server.on("connection", this._onConnection.bind(this));
    this.server.on("error", this._onError.bind(this));
    listens.push(`InputStream on ${this.port}`);
    this.server.listen(this.port);
  }

  _read() {
    this.pushing = true;
  }

  _onConnection(c) {
    winston.info("InputStream TCP connected port " + this.port);
    c.on("data", this._onData.bind(this));
    c.on("end", this._onEnd.bind(this));
    c.on("error", this._onError.bind(this));
  }

  _onData(chunk) {
    if (!this.pushing) {
      return;
    }
    const result = this.push(chunk);
    if (result === false) {
      this.pushing = false;
    }
  }

  _onEnd() {
    this.pushing = false;
    this.push(null);
  }

  _onError(err) {
    winston.error("TCP Error", err);
    process.exit(1);
  }

  stop() {
    this.pushing = false;
    if (this.server) {
      this.server.close();
    }
  }
}

export class OutputStream extends Writable {
  constructor(params = {}) {
    super();
    this.retryHandle = null;
    if (params.url) {
      this.setURL(params.url);
    }
    this.buffer = [];
    this._retry();
  }

  _retry() {
    winston.debug(`Attempting connection to ${this.hostname}:${this.port}`);
    this.conn = net.createConnection({port: this.port, host: this.hostname});
    this.conn.on("error", ::this._onError);
    this.conn.on("connect", ::this._onConnect);
    this.conn.on("end", ::this._onEnd);
    this.conn.on("close", ::this._onClose);
    this.conn.on("timeout", ::this._onTimeout);
    this.connected = false;
  }

  _throttleRetry(timeout = 3000) {
    if (this.retryHandle !== null) {
      // We already know and are auto-retrying.
      return;
    }
    this.connected = false;
    winston.debug(`Retrying in ${timeout}ms`);
    this.retryHandle = setTimeout(() => {
      this.retryHandle = null;
      this._retry();
    }, timeout);
  }

  _onError(err) {
    // If it's refused we just assume they're not up yet and poll right away.
    let timeout = 3000;
    if (err.code === "ECONNREFUSED") {
      timeout = 500;
    }
    else {
      winston.error(err);
    }
    winston.debug("Connection failure.");
    this._throttleRetry(timeout);
  }

  _onConnect(...args) {
    winston.info(`${this.port} connected`, ...args);
    this.connected = true;
  }

  _onEnd(...args) {
    winston.debug("Connection ended.", ...args);
    this._throttleRetry();
  }

  _onClose(...args) {
    winston.debug("Connection closed.", ...args);
    this._throttleRetry();
  }

  _onTimeout(...args) {
    winston.debug("Connection timed out.", ...args);
    this._throttleRetry();
  }

  setURL(newURL) {
    const {port, hostname} = url.parse(newURL);
    if (port === this.port && hostname === this.hostname) {
      // No change.
      return;
    }
    this.port = port;
    this.hostname = hostname;
    if (this.connected) {
      this.conn.end();
      this._retry();
    }
  }

  _write(chunk, encoding, done) {
    if (this.connected) {
      this.conn.write(chunk);
    }
    done();
  }

  end(...args) {
    super.end(...args);
    this.conn.end();
    if (this.retryHandle) {
      clearTimeout(this.retryHandle);
    }
  }
}


// Old, broken server-based stream
// /**
//  * UDPOutputStream implements piping all data recieved to somewhere on the internet over UDP.
//  */
// export class OutputStream extends Writable {
//   constructor(params = {}) {
//     super();
//     if (params.url) {
//       this.setURL(params.url);
//     }
//     this.buffer = [];
//     this.server = net.createServer();
//     this.server.on("connection", this._onConnection.bind(this));
//     this.server.on("listening", this._onListening.bind(this));
//     this.server.on("error", this._onError.bind(this));
//     this.connections = [];
//     listens.push(`OutputStream: ${this.port}`);
//     this.server.listen({port: this.port, host: this.hostname});
//   }

//   _onListening() {
//     winston.log(`Listening on localhost:${this.port}`);
//   }

//   _onConnection(c) {
//     c.setNoDelay(true);
//     c.on("error", this._onError.bind(this, c));
//     c.on("disconnect", this._onDisconnect.bind(this, c));
//     // If we have buffered data, push that all to the first person to connect
//     while (this.buffer.length > 0) {
//       c.write(this.buffer.pop());
//     }
//     this.connections.push(this);
//   }

//   _onDisconnect(c) {
//     const index = this.connections.indexOf(c);
//     this.connections.splice(index, 1);
//   }

//   setURL(newURL) {
//     const {port, hostname} = url.parse(newURL);
//     this.port = port;
//     this.hostname = hostname;
//   }

//   _onError(err) {
//     winston.error("TCP Error", err);
//     console.log(listens);
//     process.exit(1);
//   }

//   _write(chunk, encoding, done) {
//     // if (this.connections.length === 0) {
//     //   // No connections? Buffer our data until somebody shows up.
//     //   this.buffer.push(chunk);
//     // }
//     // else {
//       this.connections.forEach((c) => {
//         c.write(chunk, null, () => {
//           console.log("Got wrote")
//         });
//       });
//       done();
//     // }
//   }
// }
