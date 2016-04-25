/**
 * Shared behavior of all inputs. This does a little too much right now... eventually the NoSignal
 * stuff will be on the recieving end.
 *
 * TODO: handle changes on the vertices of our outputs.
 */

import dgram from "dgram";
import url from "url";
import {syncer} from "mpeg-munger";
import {SERVER_START_TIME} from "../../constants";

import NoSignalStream from "../NoSignalStream";
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

// We want to wait juuuust
const STARTUP_DELAY = 2000;

export default class InputVertex extends BaseVertex {
  constructor(params) {
    super(params);
    this.startedMultiplexing = false;
    this._udpServers = [];
  }

  init() {
    super.init();
    this.startMultiplexing(this.doc);
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

    this.doc.outputs.forEach((output) => {
      const sync = syncer({
        count: output.sockets.length,
        offset: 0,
        startTime: SERVER_START_TIME,
      });
      output.sockets.forEach((socket, i) => {
        const syncStream = sync.streams[i];
        const noSignalStream = new NoSignalStream({delay: 2000, type: socket.type});
        const {port} = url.parse(socket.url);
        const server = this._getServer();
        server.on("error", (...args) => {
          this.error(args);
        });
        server.on("message", (chunk, rdata) => {
          syncStream.write(chunk);
        });
        server.bind(port);
        syncStream.pipe(noSignalStream);
        noSignalStream.on("data", (chunk) => {

        });
      });
    });

    this.vertexHandle.then(() => {
      this.arcHandle = SK.arcs.watch({"from": {"vertexId": this.id}})
      .then((docs) => {
        // console.log(docs);
      })
      .catch((e) => {
        this.error(e);
      });
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
