
/*eslint-disable no-console */

import express from "express";
import path from "path";
import timesyncServer from "timesync/server";

const port = process.env.PORT || 9090;
const app = express();
app.listen(port);
console.log(`sk-time listening on ${port}`);

const clientScript = path.resolve(__dirname, "..", "node_modules", "timesync", "dist");
const index = path.resolve(__dirname, "static", "index.html");

// CORS
app.use(function(req, res, next) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept");
  if (req.method === "OPTIONS") {
    return res.end();
  }
  next();
});

app.use("/", express.static(index));
app.use("/dist", express.static(clientScript));
app.get("/*", express.static(path.resolve(__dirname, "static")));
app.use("/timesync", timesyncServer.requestHandler);
