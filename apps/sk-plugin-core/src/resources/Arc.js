
import Resource from "sk-resource";
import winston from "winston";

export default class Arc extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }

  /**
   * A vertex was deleted -- make sure that any arcs attached to it also get deleted.
   */
  handleVertexDeletion(ctx, vertexId) {
    if (typeof vertexId !== "string") {
      throw new Error("Missing vertexId in handleVertexDeletion");
    }
    return Promise.all([
      this.db.multiDelete(ctx, {from: {vertexId}}),
      this.db.multiDelete(ctx, {to: {vertexId}}),
    ])
    .then(([r1, r2]) => {
      winston.info(`Deleted ${r1.deleted} arcs that connected from vertex ${vertexId}`);
      winston.info(`Deleted ${r2.deleted} arcs that connected to vertex ${vertexId}`);
    });
  }
}

Arc.tableName = "arcs";
