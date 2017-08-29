import SP from "sp-client";
import config from "sp-configuration";
import winston from "winston";
import {
  tcpIngressStream,
  rtmpOutputStream,
  mpegMungerStream,
  ptsNormalizerStream
} from "sp-streams";
import { PassThrough } from "stream";
import FileStreamer from "./file-streamer";

const getTableName = objName => {
  const definition = SP.schema.definitions[objName];
  if (!definition) {
    throw new Error(`I've never heard of an ${objName}, can't watch it`);
  }
  return definition.tableName;
};

export default class SPBroadcaster {
  constructor({ broadcastId, podIp }) {
    this.podIp = podIp;
    this.mainStream = ptsNormalizerStream();
    // this.mainStream.on("pts", ({ pts }) => {
    //   console.log(pts);
    // });
    this.mainStream.resume();
    winston.info(`sp-broadcaster running for broadcast ${broadcastId}`);
    this.sourceHandles = {};
    this.sources = {};
    this.outputStreams = {};
    const broadcastHandle = SP.broadcasts
      .watch({ id: broadcastId })
      .on("data", ([broadcast]) => {
        this.broadcast = broadcast;
        this.watchSources();
      });
    const outputHandle = SP.outputs
      .watch({ broadcastId })
      .on("newDoc", output => {
        const outputStream = rtmpOutputStream({
          rtmpUrl: output.url
        });
        this.outputStreams[output.id] = outputStream;
        this.mainStream.pipe(outputStream);
      })
      .on("deletedDoc", id => {
        this.mainStream.unpipe(this.outputStreams[id]);
        this.outputStreams[id].end();
        delete this.outputStreams[id];
      });
    Promise.all([broadcastHandle, outputHandle]).then(() => {
      this.reconcile();
    });
  }

  watchSources() {
    const sourcesShouldWatch = this.broadcast.sources.map(({ kind, id }) => {
      return `${kind}/${id}`;
    });
    const sourcesCurrentlyWatching = Object.keys(this.sourceHandles);
    const startWatching = sourcesShouldWatch.filter(
      s => !sourcesCurrentlyWatching.includes(s)
    );
    const stopWatching = sourcesCurrentlyWatching.filter(
      s => !sourcesShouldWatch.includes(s)
    );
    startWatching.forEach(s => {
      const [kind, id] = s.split("/");
      const tableName = getTableName(kind);
      const handles = [];
      handles.push(
        SP[tableName].watch({ id }).on("data", ([source]) => {
          this.sources[id] = source;
        }),
        SP.streams.watch({ source: { id } }).on("newDoc", stream => {
          winston.info(
            `Boy howdy I should download some data from ${stream.url}`
          );
          const tcpIngress = tcpIngressStream({ url: stream.url });
          tcpIngress.pipe(this.mainStream, { end: false });
        })
      );
      this.sourceHandles[s] = handles;
    });
    stopWatching.forEach(s => {
      const [kind, id] = s.split("/");
      this.sourceHandles[s].forEach(handle => handle.stop());
      delete this.sourceHandles[s];
    });
  }

  reconcile() {
    winston.info(`Broadcast ${this.broadcast.id} updated`);
  }
}

if (!module.parent) {
  const BROADCAST_ID = config.require("BROADCAST_ID");
  const POD_IP = config.require("POD_IP");
  SP.connect()
    .then(() => {
      new SPBroadcaster({ broadcastId: BROADCAST_ID, podIp: POD_IP });
      // TODO: this should be a separate pod
      new FileStreamer({ broadcastId: BROADCAST_ID, podIp: POD_IP });
    })
    .catch(err => {
      winston.error(err);
      process.exit(1);
    });
}

process.on("SIGTERM", function() {
  process.exit(0);
});
