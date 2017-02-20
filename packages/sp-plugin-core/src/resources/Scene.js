
import Resource from "sk-resource";

export default class Scene extends Resource {
  auth(ctx, doc) {
    return Promise.resolve();
  }

  authQuery(ctx, query) {
    return Promise.resolve({});
  }
}

Scene.tableName = "scenes";
