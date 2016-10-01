
import EE from "events";
import r from "rethinkdb";

export default class SKContext extends EE {
  constructor() {
    super();
    this.resources = SKContext.resources;
  }

  data(tableName, oldVal, newVal) {
    if (!newVal) {
      this.emit("data", {
        tableName,
        id: oldVal.id,
        doc: null
      });
    }
    else {
      this.emit("data", {
        tableName,
        id: newVal.id,
        doc: newVal
      });
    }
  }

  cleanup() {
    if (this.conn) {
      this.conn.close();
    }
  }
}

SKContext.resources = {};
SKContext.addResource = function(resource) {
  if (SKContext.resources[resource.constructor.tableName]) {
    throw new Error(`Context got resource ${resource.constructor.name} twice!`);
  }
  SKContext.resources[resource.constructor.tableName] = resource;
};

SKContext.createContext = function({rethinkHost, rethinkPort, rethinkDatabase, token}) {
  return r.connect({
    host: rethinkHost,
    port: rethinkPort,
    db: rethinkDatabase,
  })
  .then((conn) => {
    const cxt = new SKContext();
    cxt.rethink = r;
    cxt.conn = conn;
    return cxt;
  });
};
