
/* eslint-disable no-console */
/* global onmessage */

import {resolve} from "path";
import net from "net";

console.log("Worker process started.");

let socket;
let bytes = 0;

const server = net.createServer();
server.listen(resolve("output", "output.sock"));

server.on("connection", function(s) {
  console.log("Connection opened to output.sock");
  socket = s;

  s.on("close", function() {
    socket = null;
  });
});

let last = Date.now();
setInterval(function() {
  let now = Date.now();
  let mib = bytes/1024/1024;
  let diff = (now - last) / 1000;
  console.log(`${mib / diff} MiB/s`);
  last = now;
  bytes = 0;
}, 5000);

onmessage = function(e) {
  bytes += e.data.length;
  if (socket) {
    socket.write(Buffer.from(e.data));
  }
};
