
/**
 * An arc, to which vertices may write their output. Created by a vertex output multiplexer.
 */

import winston from "winston";
import dgram from "dgram";
import url from "url";

import {UDPOutputStream} from "./UDPStreams";
import SK from "../sk";

export default class ArcWritable {
  constructor({arcId, outputs, ioName}) {
    this.arcId = arcId;
    this.ioName = ioName;
    const outputTypes = Object.keys(outputs);
    this.streams = {};
    outputTypes.forEach((type) => {
      this.streams[type] = new UDPOutputStream();
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
