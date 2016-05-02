
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
    this.info("initializing");

    // Watch my vertices, so I can create and delete as necessary.
    this.vertexHandle = SK.vertices.watch({broadcastId: this.id})

    .on("newDoc", (vertex) => {
      this.info(`initializing vertex ${vertex.id}`);
      this.vertices[vertex.id] = Vertex.create(vertex);
    })

    .on("deletedDoc", (id) => {
      this.vertices[id].cleanup();
      delete this.vertices[id];
    })

    .catch((err) => {
      this.error("Error on pulling vertices", err);
    });
  }

  cleanup() {
    Object.keys(this.vertices).forEach((id) => {
      this.vertexHandle.stop();
      this.vertices[id].cleanup();
      SK.vertices.update(id, {
        status: "INACTIVE",
        timemark: ""
      });
      delete this.vertices[id];
    });
  }
}
