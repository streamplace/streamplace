import SP from "sp-client";
import config from "sp-configuration";
import winston from "winston";

export default class SPBroadcaster {
  constructor({ broadcastId }) {
    winston.info(`sp-broadcaster running for broadcast ${broadcastId}`);
    // Do nothing forever plz
    setInterval(function() {}, 1000);
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
