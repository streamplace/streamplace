
import dgram from "dgram";
import url from "url";

import Base from "./Base";
import SK from "../sk";

export default class Arc extends Base {
  constructor(params) {
    super(params);
    // Watch my vertex, so I can respond appropriately.
    const {id, broadcast} = params;
    this.id = id;
    this.broadcast = broadcast;
    SK.arcs.watch({id: this.id})

    .on("data", ([arc]) => {
      this.doc = arc;
      if (arc) {
        this.init();
      }
      else {
        this.cleanup();
      }
    })

    .catch((err) => {
      this.error("Error on initial pull", err);
    });

    this.sendSocket = dgram.createSocket("udp4");
  }

  setupFromPipe() {
    if (this.server) {
      this.server.close();
    }
    const {port} = url.parse(this.fromSocket);
    this.server = dgram.createSocket("udp4");
    this.server.on("error", (err) => {
      this.error(err);
    });
    this.server.on("message", (msg, rinfo) => {
      if (this.sendPort) {
        this.sendSocket.send(msg, 0, msg.length, this.sendPort, "127.0.0.1");
      }
    });
    this.server.on("listening", () => {
      this.info("listening");
    });
    this.server.bind(port);
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
    if (this.server) {
      this.server.close();
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
