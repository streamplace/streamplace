
import config from "sk-config";
import winston from "winston";

import Vertex from "./classes/Vertex";
import SK from "./sk";

SK.vertices.findOne(config.require("VERTEX_ID"))
.then((vertex) => {
  Vertex.create(vertex);
})
.catch((err) => {
  winston.error(err);
  process.exit(1);
});
