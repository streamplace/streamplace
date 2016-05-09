/**
 * Shared behavior of all inputs. But not really. Really this should be its own module that runs
 * on the same pod as a vertex that handles piping its data around. It's stuck here because I
 * didn't realize that until I was finished writing it, but if you add any code here make sure you
 * don't take too much advantage of the fact that you're part of the Vertex class 'cause we want
 * this all to be moved out of this process real soon.
 *
 * TODO: handle changes on the vertices of our outputs.
 */

import dgram from "dgram";
import url from "url";
import {syncer} from "mpeg-munger";
import _ from "underscore";

import * as udp from "../transports/UDPTransport";
import {SERVER_START_TIME} from "../../constants";
import ArcWritable from "../ArcWritable";
import NoSignalStream from "../NoSignalStream";
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class InputVertex extends BaseVertex {
  constructor(params) {
    super(params);
    this.startedMultiplexing = false;
    this.arcStreams = [];
    this._udpServers = [];
  }

  init() {
    super.init();
    setTimeout(() => { //todo fixme it's 4am
      this.startMultiplexing();
    }, 2000);
  }

  /**
   * Begin monitoring our relevant arcs and sending data as appropriate.
   * @return {[type]} [description]
   */
  startMultiplexing() {
    if (this.startedMultiplexing === true) {
      return;
    }
    this.startedMultiplexing = true;
    this.info("Started multiplexing");

    this.outputStreams = {};

    // List of every syncer stream, so I can set all their offsets at once.
    this.syncers = [];

    // List of all of my streams that I need to clean up when we're done.
    this.cleanupStreams = [];

    this.doc.outputs.forEach((output) => {
      const streamsForThisOutput = {};
      this.outputStreams[output.name] = streamsForThisOutput;
      const sync = syncer({
        count: output.sockets.length,
        offset: 0,
        startTime: SERVER_START_TIME,
      });
      this.syncers.push(sync);
      output.sockets.forEach((socket, i) => {

        const {protocol} = url.parse(socket.url);
        // The data we're producing gets passed through a series of streams.
        let dataInStream; // We put our new data in this stream

        if (protocol === "udp:") {
          dataInStream = new udp.InputStream({url: socket.url});
        }
        else {
          throw new Error(`We don't know how to proxy streams for this protocol: ${protocol}`);
        }


        let dataOutStream; // And then it comes out here when we're done.

        // If we're an input-ish stream, we keep our input in sync and fill it with NoSignal.
        if (this.rewriteStream === true) {
          const syncStream = sync.streams[i];
          const noSignalStream = new NoSignalStream({delay: 2000, type: socket.type});
          this.cleanupStreams.push(noSignalStream);
          dataInStream.pipe(syncStream);
          syncStream.pipe(noSignalStream);
          dataOutStream = noSignalStream;
        }

        // Otherwise, we just go in one ear and out the other.
        else {
          dataOutStream = dataInStream;
        }

        // Let's save the output stream so we can pass it to all applicable arcs later.
        streamsForThisOutput[socket.type] = dataOutStream;
      });
    });

    this.vertexHandle.then(() => {
      this.arcHandle = SK.arcs.watch({"from": {"vertexId": this.id}})
      .on("newDoc", (arc) => {
        this.addArc(arc);
      })
      .on("deletedDoc", (id) => {
        this.removeArc(id);
      })
      .on("data", (arcs) => {
        // For now we're just taking the delay from the first arc we see.
        const arc = arcs[0];
        if (!arc) {
          return;
        }
        const offset = parseInt(arc.delay);
        this.syncers.forEach((sync) => {
          sync.setOffset(offset);
        });
      })
      .catch((e) => {
        this.error(e);
      });
    });
  }

  addArc(arc) {
    const sockets = this.outputStreams[arc.from.ioName];
    const arcWritable = new ArcWritable({
      arcId: arc.id,
      ioName: arc.from.ioName,
      outputs: sockets
    });
    this.arcStreams.push(arcWritable);
    Object.keys(sockets).forEach((type) => {
      sockets[type].pipe(arcWritable.streams[type]);
    });
  }

  removeArc(arcId) {
    const arcWritable = _(this.arcStreams).findWhere({arcId});
    const sockets = this.outputStreams[arcWritable.ioName];
    Object.keys(sockets).forEach((type) => {
      sockets[type].unpipe(arcWritable.streams[type]);
    });
  }

  _getServer() {
    const server = dgram.createSocket("udp4");
    this._udpServers.push(server);
    return server;
  }

  cleanup() {
    super.cleanup();
    if (this.arcHandle) {
      this.arcHandle.stop();
    }
    if (this.vertexHandle) {
      this.vertexHandle.stop();
    }
    this.cleanupStreams.forEach((stream) => {
      stream.end();
    });
    this._udpServers.forEach((server) => {
      server.close();
    });
  }
}
