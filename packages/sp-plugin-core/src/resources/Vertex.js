import Resource from "sp-resource";

export default class Vertex extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }

  delete(ctx, id) {
    return super.delete(ctx, id).then(() => {
      return ctx.resources.arcs.handleVertexDeletion(ctx, id);
    });
  }
}

Vertex.tableName = "vertices";
