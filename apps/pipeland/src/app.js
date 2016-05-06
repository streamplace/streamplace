
import winston from "winston";
import _ from "underscore";

import Broadcast from "./classes/Broadcast";
import ENV from "./env";
import SK from "./sk";

winston.cli();
winston.info("Pipeland starting up.");

// Main loop. Eventually this will be replaced with a scheduler that allocates vertices onto
// Kubernetes nodes. For now we just follow them all here.
const broadcasts = {};
SK.broadcasts.watch({enabled: true})
.on("newDoc", (broadcast) => {
  broadcasts[broadcast.id] = new Broadcast(broadcast);
})
.on("deletedDoc", (id) => {
  broadcasts[id].cleanup();
  delete broadcasts[id];
});
