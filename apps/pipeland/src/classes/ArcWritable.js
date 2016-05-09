
/**
 * An arc, to which vertices may write their output. Lots of these are created by something that's
 * multiplexing outputs from a vertex.
 *
 * Current limitation: you can't change protocols on the fly. You can change URLs on the fly, just
 * not protocols.
 */

import winston from "winston";
import dgram from "dgram";
import url from "url";
import {PassThrough} from "stream";

import * as udp from "./transports/UDPTransport";
import SK from "../sk";

export default class ArcWritable {
  constructor({arcId, outputs, ioName}) {
    this.arcId = arcId;
    this.ioName = ioName;
    this.outputTypes = Object.keys(outputs);
    this.streams = {};
    this.outputStreams = {};

    this.outputTypes.forEach((type) => {
      this.streams[type] = new PassThrough();
      this.outputStreams[type] = null;
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
    // Our destination vertex changed. Stop listening to the old one.
    if (this.vertexHandle) {
      this.vertexHandle.stop();
    }
    // Our destination vertex changed. Stop redirecting data to those old sockets.
    this.outputTypes.forEach((type) => {
      if (this.outputStreams[type] !== null) {
        this.streams[type].unpipe(this.outputStreams[type]);
        this.outputStreams[type] = null;
      }
      this.outputStreams[type] = null;
    });
    this.vertexHandle = SK.vertices.watch({id: this.doc.to.vertexId})
    .on("data", ([vertex]) => {
      const [input] = vertex.inputs.filter(input => input.name === this.doc.to.ioName);
      input.sockets.forEach((socket, i) => {
        if (!this.streams[socket.type]) {
          return; // No problem -- our output takes more streams than our input has.
        }
        if (!this.outputStreams[socket.type]) {
          // It's our first time here! Create the output.
          const {protocol} = url.parse(socket.url);
          if (protocol === "udp:") {
            this.outputStreams[socket.type] = new udp.OutputStream({url: socket.url});
            this.streams[socket.type].pipe(this.outputStreams[socket.type]);
          }
          else {
            throw new Error(`Trying to arc to an unknown protocol type: ${protocol}`);
          }
        }
        this.outputStreams[socket.type].setURL(socket.url);
      });
    })
    .on("deleted", () => {
      // TODO: this should have a higher error class when such a thing exists.
      winston.error(`Vertex ${this.vertex.id} was deleted, but it's still the destination of arc ${this.arcId}!`);
    })
    .catch((e) => {
      winston.error(`Arc ${this.arcId} errored while looking for destinationVertex ${this.doc.to.vertexId}`);
    });
  }
}
