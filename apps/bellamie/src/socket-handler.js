
import SocketIO from "socket.io";
import {SKContext} from "sk-resource";
import config from "sk-config";
import winston from "winston";
import _ from "underscore";
import querystring from "querystring";
import url from "url";

const RETHINK_HOST = config.require("RETHINK_HOST");
const RETHINK_PORT = config.require("RETHINK_PORT");
const RETHINK_DATABASE = config.require("RETHINK_DATABASE");

export default function(server) {
  const io = SocketIO(server);

  io.use(function(socket, next){
    const {query} = url.parse(socket.request.url);
    const {token} = querystring.parse(query);
    SKContext.createContext({
      token: token,
      rethinkHost: RETHINK_HOST,
      rethinkPort: RETHINK_PORT,
      rethinkDatabase: RETHINK_DATABASE
    }).then((ctx) => {
      socket.ctx = ctx;
      next();
    })
    .catch((err) => {
      next(new Error(JSON.stringify({
        status: err.status,
        code: err.code,
        message: err.message,
      })));
    });
  });

  io.on("connection", function(socket) {
    const ctx = socket.ctx;
    const handles = {};
    const addr = socket.conn.remoteAddress;
    winston.info(`HELLO ${addr}`);
    socket.emit("hello");
    socket.on("disconnect", () => {
      winston.info(`DISCONNECT ${addr}`);
      Promise.all(_(handles).values().map((handle) => {
        return handle.close();
      })).then(() => {
        socket.ctx.cleanup();
      });
    });

    ctx.on("data", socket.emit.bind(socket, "data"));

    socket.on("sub", function({subId, resource, query = {}}) {
      if (!subId || handles[subId]) {
        throw new Error(`Invalid subId = ${subId}`);
      }
      winston.info(`SUB (${subId}) ${addr} '${resource}' WHERE ${JSON.stringify(query)}`);
      ctx.resources[resource].watch(ctx, query).then((handle) => {
        handles[subId] = handle;
        socket.emit("suback", {subId});
      });
    });

    socket.on("unsub", function({subId}) {
      winston.info(`UNSUB (${subId}) ${addr}`);
      if (!handles[subId]) {
        return winston.error(`UNSUB on nonexistent subId ${subId}`);
      }
      handles[subId].close();
      delete handles[subId];
    });
  });

  return io;
}
