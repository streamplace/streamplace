import SP from "sp-client";
import config from "sp-configuration";
import winston from "winston";

const getTableName = objName => {
  const definition = SP.schema.definitions[objName];
  if (!definition) {
    throw new Error(`I've never heard of an ${objName}, can't watch it`);
  }
  return definition.tableName;
};

export default class SPBroadcaster {
  constructor({ broadcastId }) {
    winston.info(`sp-broadcaster running for broadcast ${broadcastId}`);
    this.sourceHandles = {};
    this.sources = {};
    const broadcastHandle = SP.broadcasts
      .watch({ id: broadcastId })
      .on("data", ([broadcast]) => {
        this.broadcast = broadcast;
        this.watchSources();
      });
    const outputHandle = SP.outputs
      .watch({ broadcastId })
      .on("data", outputs => {
        this.outputs = outputs;
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
      this.sourceHandles[s] = SP[tableName]
        .watch({ id })
        .on("data", ([source]) => {
          this.sources[id] = source;
        });
    });
    stopWatching.forEach(s => {
      const [kind, id] = s.split("/");
      this.sourceHandles[s].stop();
      delete this.sourceHandles[s];
    });
  }

  reconcile() {
    winston.info(`Broadcast ${this.broadcast.id} updated`);
  }
}

if (!module.parent) {
  const BROADCAST_ID = config.require("BROADCAST_ID");
  SP.connect()
    .then(() => {
      new SPBroadcaster({ broadcastId: BROADCAST_ID });
    })
    .catch(err => {
      winston.error(err);
      process.exit(1);
    });
}

process.on("SIGTERM", function() {
  process.exit(0);
});
