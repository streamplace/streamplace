
import Resource from "../resource";
import r from "rethinkdb";
import winston from "winston";

export default class Vertex extends Resource {
  constructor() {
    super("vertices");
  }

  beforeDelete(id, conn) {
    return super.beforeDelete()
    .then(() => {
      return r.table("arcs").filter({from: {vertexId: id}}).delete().run(conn);
    })
    .then((result) => {
      winston.info(`Deleted ${result.deleted} arcs that connected from vertex ${id}`);
      return r.table("arcs").filter({to: {vertexId: id}}).delete().run(conn);
    })
    .then((result) => {
      winston.info(`Deleted ${result.deleted} arcs that connected to vertex ${id}`);
    });
  }
}
