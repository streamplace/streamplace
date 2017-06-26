/* eslint-disable no-console */
/* global onmessage */

import { resolve } from "path";
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

const FRAMERATE = 30;
let startedEmitting;
let frameCount = 0;

let last = performance.now();
setInterval(function() {
  let now = performance.now();
  // let mib = bytes/1024/1024;
  // let diff = (now - last) / 1000;
  // console.log(`${mib / diff} MiB/s`);
  last = now;
  bytes = 0;
  if (startedEmitting) {
    const desiredCount = (now - startedEmitting) / 1000 * FRAMERATE;
    const offset = desiredCount - frameCount;
    if (Math.abs(offset) > 1.5) {
      console.log(`Output framerate is off by ${offset}`);
    }
  }
}, 5000);

let frame;

const run = function() {
  setTimeout(run, 5);
  if (!socket) {
    return;
  }
  const now = performance.now();
  if (!startedEmitting) {
    startedEmitting = now;
  }
  let desiredCount = (now - startedEmitting) / 1000 * FRAMERATE;
  if (frameCount > desiredCount) {
    return;
  }
  bytes += frame.length;
  frameCount += 1;
  socket.write(Buffer.from(frame));
};

let started = false;
onmessage = function(e) {
  frame = e.data;
  if (!started) {
    started = true;
    run();
  }
};
