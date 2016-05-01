
import winston from "winston";
import _ from "underscore";

import Vertex from "./classes/Vertex";
import ENV from "./env";
import SK from "./sk";

winston.cli();
winston.info("Pipeland starting up.");

const vertices = {};

// Main loop. Eventually this will be replaced with a scheduler that allocates vertices onto
// Kubernetes nodes. For now we just follow them all here.
SK.vertices.watch({}).then(function(vertices) {
  winston.info(`Got ${vertices.length} vertices in the initial pull.`);
  vertices.forEach(function(vertex) {
    vertices[vertex.id] = Vertex.create(vertex);
  });
})
.on("created", (docs) => {
  docs.forEach((doc) => {
    winston.info(`Initializing vertex ${doc.id}`);
    vertices[doc.id] = Vertex.create({id: doc.id});
  });
})
.catch(function(err) {
  winston.error("Error getting vertices", err);
});
