
import dgram from "dgram";
import url from "url";

import PortManager from "./PortManager";
import Base from "./Base";
import SK from "../sk";

export default class Arc extends Base {
  constructor(params) {
    super(params);
    // Watch my vertex, so I can respond appropriately.
    const {id, broadcast} = params;
    this.id = id;
    this.broadcast = broadcast;
    // Bind all of our handlers, so that we can `.on` and `.removeListener` them with impunity
    this.handleMessage = this.handleMessage.bind(this);
    this.handleError = this.handleError.bind(this);

    // Watch our arc!
    SK.arcs.watch({id: this.id})
    .on("data", ([arc]) => {
      this.doc = arc;
      if (arc) {
        this.init();
      }
      else {
        this.closeListener();
        this.cleanup();
      }
    })

    .catch((err) => {
      this.error("Error on initial pull", err);
    });

    this.sendSocket = dgram.createSocket("udp4");
  }

  setupFromPipe() {
    this.closeListener();
    const {port} = url.parse(this.fromSocket);
    this.server = PortManager.createSocket(port);
    this.server.on("error", this.handleError);
    this.server.on("message", this.handleMessage);
  }

  closeListener() {
    if (this.server) {
      this.server.removeListener("error", this.handleError);
      this.server.removeListener("message", this.handleMessage);
      this.server.close();
      this.server = null;
    }
  }

  handleError(err) {
    this.error(err);
  }

  handleMessage(msg, rinfo) {
    if (this.sendPort) {
      this.sendSocket.send(msg, 0, msg.length, this.sendPort, "127.0.0.1");
    }
  }

  setupToPipe() {
    const {port} = url.parse(this.toSocket);
    this.sendPort = parseInt(port);
  }

  cleanup() {
    if (this.fromHandle) {
      this.fromHandle.stop();
    }
    if (this.toHandle) {
      this.toHandle.stop();
    }
  }

  init() {
    this.cleanup();
    this.fromHandle = SK.vertices.watch({id: this.doc.from.vertexId})
    .on("data", ([vertex]) => {
      const fromSocket = vertex.outputs[this.doc.from.output].socket;
      if (fromSocket && this.fromSocket !== fromSocket) {
        this.fromSocket = fromSocket;
        this.setupFromPipe();
      }
    })
    .catch((err) => {
      this.error(err);
    });

    this.toHandle = SK.vertices.watch({id: this.doc.to.vertexId})
    .on("data", ([vertex]) => {
      const toSocket = vertex.inputs[this.doc.to.input].socket;
      if (toSocket && this.toSocket !== toSocket) {
        this.toSocket = toSocket;
        this.setupToPipe();
      }
    })
    .catch((err) => {
      this.error(err);
    });
  }
}

Arc.create = function(params) {
  return new Arc(params);
};
