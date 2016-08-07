
/*eslint-disable no-console */

import express from "express";
import path from "path";
import timesyncServer from "timesync/server";
import SocketIO from "socket.io";
import SKClient from "sk-client";
import http from "http";
import url from "url";
import querystring from "querystring";
import winston from "winston";

const SK = new SKClient();
const port = process.env.PORT || 9090;
const app = express();
const server = http.createServer(app);
const clientScript = path.resolve(__dirname, "..", "node_modules", "timesync", "dist");
const index = path.resolve(__dirname, "static", "index.html");
const io = SocketIO(server);

// CORS
app.use(function(req, res, next) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept");
  if (req.method === "OPTIONS") {
    return res.end();
  }
  next();
});

// Just wraps Date.now for now. Maybe in the future we're wired up to NTP or something???
const now = function() {
  return Date.now();
};

io.use(function(socket, next){
  const {query} = url.parse(socket.request.url);
  const {overlayKey} = querystring.parse(query);
  SK.inputs.find({overlayKey})
  .then(([input]) => {
    if (!input) {
      return next(new Error("Input not found"));
    }
    socket.input = input;
    next();
  })
  .catch((err) => {
    next(err);
  });
});

io.on("connection", function(socket) {
  socket.emit("hello");
  socket.on("timesync", function(message, reply) {
    reply(now());
  });
  const inputHandle = SK.inputs.watch({id: socket.input.id})
  .catch(::winston.error)
  .on("data", ([input]) => {
    if (input.nextSync) {
      socket.emit("nextsync", input.nextSync);
    }
  });
  socket.on("disconnect", () => {
    inputHandle.stop();
  });
});

app.use("/", express.static(index));
app.use("/dist", express.static(clientScript));
app.get("/*", express.static(path.resolve(__dirname, "static")));
app.use("/timesync", timesyncServer.requestHandler);

server.listen(port);
console.log(`sk-time listening on ${port}`);
