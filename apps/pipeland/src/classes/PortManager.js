/**
 * This is the guy that handles our UDP ports. If four arcs need to listen on the same port, it
 * listens once and multiplexes to the four. Once all four hang up, it stops listening. It's
 * badass like that.
 *
 * Also, because we're in the neighborhood, it handles allocating ports for everything.
 */

import dgram from "dgram";
import EE from "events";
import winston from "winston";

/**
 * Most stuff is only going to interact with the static methods at the bottom of this file. That
 * will return a PortManager instance as necessary.
 */
export default class PortManager extends EE {
  constructor(port) {
    super();
    // Count of how many arcs are listening on our port. We can close our server when it's zero.
    this.listeners = 0;
    this.port = port;
    winston.info(`Opening port ${this.port}`);
    this.server = dgram.createSocket("udp4");
    this.server.on("error", (...args) => {
      this.emit("error", ...args);
    });
    this.server.on("message", (...args) => {
      this.emit("message", ...args);
    });
    this.server.on("listening", (...args) => {
      this.emit("listening");
    });
    this.server.bind(this.port);
  }

  incrementListeners() {
    this.listeners += 1;
  }

  close() {
    this.listeners -= 1;
    if (this.listeners < 1) {
      winston.info(`All listeners closed, closing port ${this.port}`);
      delete PortManager.activeListens[`${this.port}`];
      this.server.close();
    }
  }
}

PortManager.activeListens = {};

PortManager.createSocket = function(port) {
  const portStr = `${port}`;
  if (!PortManager.activeListens[portStr]) {
    PortManager.activeListens[portStr] = new PortManager(port);
  }
  const portManager = PortManager.activeListens[portStr];
  portManager.incrementListeners();
  return portManager;
};
