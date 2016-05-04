
/**
 * An arc, to which vertices may write their output. Created by a vertex output multiplexer.
 */

import {Writable} from "stream";
import winston from "winston";
import dgram from "dgram";
import url from "url";

import SK from "../sk";

class ArcWritableStream extends Writable {
  constructor() {
    super();
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

export default class ArcWritable {
  constructor({arcId, outputs, ioName}) {
    this.arcId = arcId;
    this.ioName = ioName;
    const outputTypes = Object.keys(outputs);
    this.streams = {};
    outputTypes.forEach((type) => {
      this.streams[type] = new ArcWritableStream();
    });

    this.arcHandle = SK.arcs.watch({id: this.arcId})
    .then(([arc]) => {
      this.doc = arc;
      this.initVertex();
    })
    .on("data", ([arc]) => {
      // If we're new or things changed, update our output.
      const oldArc = this.doc;
      this.doc = arc;
      if (!oldArc || oldArc.to.vertexId !== arc.to.vertexId || oldArc.to.ioName !== arc.to.ioName) {
        this.initVertex();
      }
    })
    .on("deleted", () => {
      if (this.vertexHandle) {
        this.vertexHandle.stop();
      }
      this.arcHandle.stop();
    });
  }

  initVertex() {
    if (this.vertexHandle) {
      this.vertexHandle.stop();
    }
    this.vertexHandle = SK.vertices.watch({id: this.doc.to.vertexId})
    .on("data", ([vertex]) => {
      const [input] = vertex.inputs.filter(input => input.name === this.doc.to.ioName);
      input.sockets.forEach((socket, i) => {
        if (!this.streams[socket.type]) {
          return; // No problem -- our output takes more streams than our input has.
        }
        this.streams[socket.type].setURL(socket.url);
      });
    })
    .on("deleted", () => {
      winston.error(`Vertex ${this.vertex.id} was deleted, but there's still an arc for it!`);
    });
  }
}
