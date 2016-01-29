
import http from "http";
import r from "rethinkdb";
import winston from "winston";

import ENV from "./env";

const config = {
  host: ENV.RETHINK_HOST,
  port: ENV.RETHINK_PORT,
  db: ENV.RETHINK_DATABASE,
};

export function* rethinkConnect(next) {
  try{
    var conn = yield r.connect(config);
    this.conn = conn;
  }
  catch(err) {
    this.status = 500;
  }
  yield next;
}

export function* rethinkClose() {
  if (this.conn) {
    this.conn.close();
  }
  else {
    winston.info("Not closing rethink connection, it doesn't appear to be open.");
  }
}
