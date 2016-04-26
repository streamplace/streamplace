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

    this.outputStreams = [];
    this.syncers = [];

    this.doc.outputs.forEach((output) => {
      const sync = syncer({
        count: output.sockets.length,
        offset: 0,
        startTime: SERVER_START_TIME,
      });
      this.syncers.push(sync);
      output.sockets.forEach((socket, i) => {
        let dataInStream;
        let dataOutStream;
        if (this.rewriteStream === true) {
          const syncStream = sync.streams[i];
          const noSignalStream = new NoSignalStream({delay: 2000, type: socket.type});
          syncStream.pipe(noSignalStream);
          dataInStream = syncStream;
          dataOutStream = noSignalStream;
        }
        else {
          dataInStream = new PassThrough();
          dataOutStream = dataInStream;
        }
        this.outputStreams.push(dataOutStream);
        const {port} = url.parse(socket.url);
        const server = this._getServer();
        server.on("error", (...args) => {
          this.error(args);
        });
        server.on("message", (chunk, rdata) => {
          dataInStream.write(chunk);
        });
        server.bind(port);
      });
    });

    this.vertexHandle.then(() => {
      this.arcHandle = SK.arcs.watch({"from": {"vertexId": this.id}})
      .then((arcs) => {
        arcs.forEach((arc) => {
          this.addArc(arc);
        });
      })
      .on("created", (arcs, newIds) => {
        // Only add arcs that are in the newIds array
        arcs.forEach((arc) => {
          if (newIds.indexOf(arc.id) === -1) {
            return;
          }
          this.addArc(arc);
        });
      })
      .on("data", ([arc]) => {
        // For now we're just taking the delay from the first arc we see.
        this.syncers.forEach((sync) => {
          sync.setOffset(parseInt(arc.delay));
        });
      })
      .on("deleted", (arcs, deletedIds) => {
        deletedIds.forEach((id) => {
          this.removeArc(id);
        });
      })
      .catch((e) => {
        this.error(e);
      });
    });
  }

  addArc(arc) {
    const arcWritable = new ArcWritable({arcId: arc.id, count: this.outputStreams.length});
    this.arcStreams.push(arcWritable);
    this.outputStreams.forEach((outputStream, i) => {
      outputStream.pipe(arcWritable.streams[i]);
    });
  }

  removeArc(arcId) {
    const arcWritable = _(this.arcStreams).findWhere({arcId});
    this.outputStreams.forEach((outputStream, i) => {
      outputStream.unpipe(arcWritable.streams[i]);
    });
  }

  _getServer() {
    const server = dgram.createSocket("udp4");
    this._udpServers.push(server);
    return server;
  }

  cleanup() {
    this.arcHandle.stop();
    this._udpServers.forEach((server) => {
      server.close();
    });
  }
}
