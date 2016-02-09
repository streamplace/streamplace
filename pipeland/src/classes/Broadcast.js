
import winston from "winston";

import SK from "../sk";
import Base from "./Base";
import Vertex from "./Vertex";
import Arc from "./Arc";

export default class Broadcast extends Base {
  constructor({id}) {
    super();
    this.id = id;
    this.vertices = {};
    this.arcs = {};
    this.info("initializing");

    // Watch my broadcast, so I can respond appropriately.
    SK.broadcasts.watch({id: this.id})

    .then((docs) => {
      this.doc = docs[0];
      this.info("Got initial pull.");
    })

    .catch((err) => {
      this.error("Error on initial pull", err);
    });

    // Watch my vertices, so I can create and delete as necessary.
    SK.vertices.watch({broadcastId: this.id})

    .then((vertices) => {
      this.info(`initializing ${vertices.length} vertices`);
      vertices.forEach((vertex) => {
        this.vertices[vertex.id] = Vertex.create({
          id: vertex.id,
          type: vertex.type,
          broadcast: this,
        });
      });
    })

    .catch((err) => {
      this.error("Error on pulling vertices", err);
    });

    // Watch my vertices, so I can create and delete as necessary.
    SK.arcs.watch({broadcastId: this.id})

    .then((arcs) => {
      this.info(`initializing ${arcs.length} arcs`);
      arcs.forEach((arc) => {
        this.arcs[arc.id] = Arc.create({
          id: arc.id,
          broadcast: this,
        });
      });
    })

    .catch((err) => {
      this.error("Error on pulling vertices", err);
    });
  }

  getVertex(vertexId) {
    if (this.vertices[vertexId]) {
      return this.vertices[vertexId];
    }
    throw new Error("Broadcast doesn't have vertex " + vertexId);
  }
}

Broadcast.create = function({id}) {
  return new Broadcast({id});
};
