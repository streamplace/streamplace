import r from "rethinkdb";
import winston from "winston";

export function ensureTableExists(name, conn) {
  return new Promise(function(resolve, reject) {
    r
      .tableCreate(name)
      .run(conn)
      .then(function() {
        winston.info(`Created table ${name}`);
        resolve();
      })
      .catch(function(err) {
        // Already exists, that's fine.
        if (err.msg.indexOf("already exists") !== -1) {
          r.table(name).wait().run(conn).then(resolve).catch(reject);
        } else {
          reject(err);
        }
      });
  });
}

export function ensureDatabaseExists(name, conn) {
  return new Promise(function(resolve, reject) {
    r
      .dbCreate(name)
      .run(conn)
      .then(function() {
        winston.info(`Created database ${name}`);
        resolve();
      })
      .catch(function(err) {
        // Already exists, that's fine.
        if (err.msg.indexOf("already exists") !== -1) {
          resolve();
        } else {
          reject(err);
        }
      });
  });
}
