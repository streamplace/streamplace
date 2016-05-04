/**
 * Shared behavior of all inputs. This does a little too much right now... eventually the NoSignal
 * stuff will be on the recieving end.
 *
 * TODO: handle changes on the vertices of our outputs.
 */

import dgram from "dgram";
import url from "url";
import {syncer} from "mpeg-munger";
import _ from "underscore";
import {PassThrough} from "stream";

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

        // The data we're producing gets passed through a series of streams.
        let dataInStream; // We put our new data in this stream
        let dataOutStream; // And then it comes out here when we're done.

        // If we're an input-ish stream, we keep our input in sync and fill it with NoSignal.
        if (this.rewriteStream === true) {
          const syncStream = sync.streams[i];
          const noSignalStream = new NoSignalStream({delay: 2000, type: socket.type});
          this.cleanupStreams.push(noSignalStream);
          syncStream.pipe(noSignalStream);
          dataInStream = syncStream;
          dataOutStream = noSignalStream;
        }

        // Otherwise, we just go in one ear and out the other.
        else {
          dataInStream = new PassThrough();
          dataOutStream = dataInStream;
        }

        // Cool, with those set up, let's make a UDP server that passes to the input stream.
        const {port} = url.parse(socket.url);
        const server = this._getServer();
        server.on("error", (...args) => {
          this.error(args);
        });
        server.on("message", (chunk, rdata) => {
          dataInStream.write(chunk);
        });
        server.bind(port);

        // And let's save the output stream so we can pass it to all applicable arcs later.
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
      .on("data", ([arc]) => {
        // For now we're just taking the delay from the first arc we see.
        this.syncers.forEach((sync) => {
          sync.setOffset(parseInt(arc.delay));
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
