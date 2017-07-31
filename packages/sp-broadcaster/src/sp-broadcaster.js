import SP from "sp-client";
import config from "sp-configuration";
import winston from "winston";

export default class SPBroadcaster {
  constructor({ broadcastId }) {
    winston.info(`sp-broadcaster running for broadcast ${broadcastId}`);
    // Do nothing forever plz
    setInterval(function() {}, 1000);
    const broadcastHandle = SP.broadcasts
      .watch({ id: broadcastId })
      .on("data", ([broadcast]) => {
        this.broadcast = broadcast;
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
