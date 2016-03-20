
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

    .on("data", (docs) => {
      this.doc = docs[0];
      this.info("Got initial pull.");
      this.init();
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
    const {port} = url.parse(this.fromPipe);
    this.server = dgram.createSocket("udp4");
    this.server.on("error", (err) => {
      this.error(err);
    });
    this.server.on("message", (msg, rinfo) => {
      this.sendSocket.send(msg, 0, msg.length, this.sendPort, "127.0.0.1");
    });
    this.server.on("listening", () => {
      this.info("listening");
    });
    this.server.bind(port);
  }

  setupToPipe() {
    const {port} = url.parse(this.toPipe);
    this.sendPort = parseInt(port);
  }

  init() {
    if (this.fromHandle) {
      this.fromHandle.stop();
    }
    if (this.toHandle) {
      this.toHandle.stop();
    }
    this.fromHandle = SK.vertices.watch({id: this.doc.from.vertexId})
    .on("data", ([vertex]) => {
      if (this.fromPipe !== vertex.pipe) {
        this.fromPipe = vertex.pipe;
        this.setupFromPipe();
      }
    })
    .catch((err) => {
      this.error(err);
    });

    this.toHandle = SK.vertices.watch({id: this.doc.to.vertexId})
    .on("data", ([vertex]) => {
      if (this.toPipe !== vertex.pipe) {
        this.toPipe = vertex.pipe;
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
