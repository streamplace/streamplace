
import winston from "winston";

import ENV from "./env";
import SK from "./sk";

const broadcastWatcher = SK.broadcasts.watch({}).then(function() {
  winston.info(`Successfully connected to Bellamie at ${ENV.BELLAMIE_SERVER}`);
});

// Main loop. Watch for broadcasts that I'm supposed to manage.
