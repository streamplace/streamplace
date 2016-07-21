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
import MpegMungerStream from "mpeg-munger";
import _ from "underscore";

import {getTransportFromURL} from "../transports";
import {SERVER_START_TIME} from "../../constants";
import ArcWritable from "../ArcWritable";
import NoSignalStream from "../NoSignalStream";
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

const ASSUME_STREAM_IS_DEAD = 10000;

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

    // List of all of my streams that I need to clean up when we're done.
    this.cleanupStreams = [];

    this.vertexWithSockets.outputs.forEach((output) => {
      const streamsForThisOutput = {};
      this.outputStreams[output.name] = streamsForThisOutput;
      output.sockets.forEach((socket, i) => {

        // The data we're producing gets passed through a series of streams.

        const transport = getTransportFromURL(socket.url);
        let dataInStream = new transport.InputStream({url: socket.url});

        let currentStream = dataInStream; // And then it comes out here when we're done.

        this.streamFilters.forEach((filter) => {
          if (filter === "nosignal") {
            const noSignalStream = new NoSignalStream({delay: ASSUME_STREAM_IS_DEAD, type: socket.type});
            currentStream.pipe(noSignalStream);
            currentStream = noSignalStream;
          }
          else if (filter === "notifypts") {
            const mpegStream = new MpegMungerStream();
            currentStream.pipe(mpegStream);
            currentStream = mpegStream;
            mpegStream.transformPTS = (pts) => {
              this.notifyPTS(pts, socket.type);
              return pts;
            };
          }
        });

        // Let's save the output stream so we can pass it to all applicable arcs later.
        streamsForThisOutput[socket.type] = currentStream;
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

  cleanup() {
    super.cleanup();
    if (this.arcHandle) {
      this.arcHandle.stop();
    }
    if (this.vertexHandle) {
      this.vertexHandle.stop();
    }
    if (this.cleanupStreams) {
      this.cleanupStreams.forEach((stream) => {
        stream.end();
      });
    }
  }
}
