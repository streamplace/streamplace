
import dgram from "dgram";
import url from "url";

import PortManager from "./PortManager";
import Base from "./Base";
import UDPBuffer from "./UDPBuffer";
import SK from "../sk";

export default class Arc extends Base {
  constructor(params) {
    super(params);
    // Watch my vertex, so I can respond appropriately.
    const {id, broadcast} = params;
    this.id = id;
    this.broadcast = broadcast;

    // Set up our UDP buffer
    this.buffer = new UDPBuffer({delay: 0});

    // Bind all of our handlers, so that we can `.on` and `.removeListener` them with impunity
    this.handleSocketMessage = this.handleSocketMessage.bind(this);
    this.handleError = this.handleError.bind(this);
    this.handleBufferMessage = this.handleBufferMessage.bind(this);
    this.handleBufferInfo = this.handleBufferInfo.bind(this);

    this.buffer.on("message", this.handleBufferMessage);
    this.buffer.on("info", this.handleBufferInfo);

    // Watch our arc!
    SK.arcs.watch({id: this.id})
    .then(([arc]) => {
      this.doc = arc;
      this.buffer.setDelay(arc.delay);
      this.init();
    })
    .on("updated", ([arc]) => {
      if (this.doc.delay !== arc.delay) {
        this.buffer.setDelay(arc.delay);
      }
      // If the vertices that we're connecting changed, reinit.
      const shouldReinit =
        arc.from.vertexId !== this.doc.from.vertexId ||
        arc.to.vertexId !== this.doc.to.vertexId;
      this.doc = arc;
      if (shouldReinit) {
        this.init();
      }
    })
    .on("deleted", () => {
      this.closeListener();
      this.cleanup();
      this.buffer.removeListener("message", this.handleBufferMessage);
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
    this.server.on("message", this.handleSocketMessage);
  }

  closeListener() {
    if (this.server) {
      this.server.removeListener("error", this.handleError);
      this.server.removeListener("message", this.handleSocketMessage);
      this.server.close();
      this.server = null;
    }
  }

  handleError(err) {
    this.error(err);
  }

  handleSocketMessage(msg, rinfo) {
    this.buffer.push(msg);
  }

  handleBufferMessage(msg) {
    if (this.sendPort) {
      this.sendSocket.send(msg, 0, msg.length, this.sendPort, "127.0.0.1");
    }
  }

  handleBufferInfo({size}) {
    SK.arcs.update(this.id, {size});
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
