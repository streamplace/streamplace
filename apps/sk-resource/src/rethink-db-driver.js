
import winston from "winston";

const req = function() {
  throw new Error("Missing required parameter");
};

export default class RethinkDbDriver {
  constructor({tableName = req(), primaryKey = "id", indices = []} = req()) {
    this.name = tableName;
    this.primaryKey = primaryKey;
    this.indices = indices;
    this._initCalled = false;
  }

  /**
   * Wrapper to call _init such that it can be overriden by child classes.
   */
  init(ctx = req()) {
    if (this._initPromise === undefined) {
      this._initPromise = this._init(ctx);
    }
    return this._initPromise;
  }

  /**
   * Idempotent function to make sure our table exists, creating it if it doesn't, on startup. Can
   * be called at the start of every outward-facing function to queue everything up while we're
   * getting ready.
   */
  _init(ctx = req()) {
    // Wrap the tableCreate so we can unconditionally proceed
    return new Promise((resolve, reject) => {
      ctx.rethink.tableCreate(this.name, {
        primaryKey: this.primaryKey
      }).run(ctx.conn).then(resolve)
      .catch((err) => {
        if (!err.msg || err.msg.indexOf("alreadyExists") !== -1) {
          reject(err);
        }
        resolve();
      });
    })
    .then(() => {
      return ctx.rethink.table(this.name).wait().run(ctx.conn);
    })
    .then(() => {
      return ctx.rethink.table(this.name).indexList().run(ctx.conn);
    })
    .then((list) => {
      return Promise.all(this.indices
        .filter((i) => {
          return list.indexOf(i) === -1;
        })
        .map((i) => {
          return ctx.rethink.table(this.name).indexCreate(i).run(ctx.conn).then(() => {
            winston.info(`Created index ${i} on table ${this.name}`);
          });
        })
      );
    });
  }

  find(ctx = req(), query = req()) {
    return this.init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).filter(query).run(ctx.conn);
    })
    .then((cursor) => {
      return cursor.toArray();
    });
  }

  findOne(ctx = req(), id = req()) {
    return this.init(ctx)
    .then(() => {
      if (this.primaryKey === "id") {
        return ctx.rethink.table(this.name).get(id).run(ctx.conn);
      }
      return ctx.rethink.table(this.name).filter({id}).run(ctx.conn)
      .then((docs) => {
        if (docs.length > 1) {
          throw new Error(`MAYDAY Data integrity violation Multiple ${this.name} with id ${id}`);
        }
        return docs[0];
      });
    });
  }

  insert(ctx = req(), doc = req()) {
    return this.init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).insert(doc).run(ctx.conn);
    })
    .then((response) => {
      if (response.errors > 0) {
        throw new Error(response.first_error);
      }
      if (!doc.id) {
        if (!response.generated_keys) {
          throw new Error("Unable to figure out the id for newly created document");
        }
        doc.id = response.generated_keys[0];
      }
      return doc;
    });
  }

  replace(ctx = req(), id = req(), doc = req()) {
    return this.init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).filter({id}).replace(doc).run(ctx.conn);
    })
    .then((response) => {
      if (response.errors > 0) {
        throw new Error(response.first_error);
      }
      return doc;
    });
  }

  delete(ctx = req(), id = req()) {
    return this.init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).get(id).delete().run(ctx.conn);
    });
  }

  multiDelete(ctx = req(), query = req()) {
    return this.init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).filter(query).delete().run(ctx.conn);
    });
  }

  watch(ctx = req(), query = req()) {
    return this.init(ctx)
    .then(() => {
      return ctx.rethink.table(this.name).filter(query).changes({
        includeTypes: true,
        includeInitial: true,
        includeStates: true
      }).run(ctx.conn);
    });
  }
}
