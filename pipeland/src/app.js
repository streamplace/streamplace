
import winston from "winston";
import _ from "underscore";

import Broadcast from "./classes/Broadcast";
import ENV from "./env";
import SK from "./sk";

const broadcasts = {};

winston.cli();
winston.info("Pipeland starting up.");

SK.broadcasts.watch({}).then(function(docs) {
  const ids = _(docs).pluck("id");
  winston.info(`Got ${ids.length} broadcasts in the initial pull.`);
  ids.forEach(function(id) {
    broadcasts[id] = Broadcast.create({id});
  });
})
.catch(function(err) {
  winston.error("Error getting broadcasts", err);
});

// Main loop. Watch for broadcasts that I'm supposed to manage.

