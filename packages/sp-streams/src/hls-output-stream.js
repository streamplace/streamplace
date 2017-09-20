import socketEgressStream from "./socket-egress-stream";
import debug from "debug";
import ffmpeg from "./ffmpeg";
import os from "os";
import path from "path";
import fs from "fs-extra";
import express from "express";

const log = debug("sp:hls-stream");

export default function() {
  const app = express();
  const segmentDir = path.resolve(
    os.tmpdir(),
    Date.now() + Math.floor(Math.random() * 1000)
  );
  app.use(express.static(segmentDir));
  const socketEgress = new socketEgressStream();
  const instance = ffmpeg()
    .input(`unix://${socketEgress.path}`)
    .inputOptions(["-probesize 60000000", "-analyzeduration 10000000"])
    .inputFormat("mpegts")
    .videoCodec("copy")
    .audioCodec("copy")
    .outputFormat("hls")
    .outputOptions(["-hls_flags delete_segments"])
    // Video out
    .output(path.resolve(segmentDir, "stream.m3u8"));
  socketEgress.outputDir = segmentDir;

  let resolvePort;
  const portProm = new Promise((resolve, reject) => {
    resolvePort = resolve;
  });
  const getPort = () => portProm;

  fs
    .ensureDir(segmentDir)
    .then(() => {
      return socketEgress.getPath();
    })
    .then(() => {
      instance.run();
    })
    .then(() => {
      return new Promise((resolve, reject) => {
        const listener = app.listen(() => {
          resolve(listener.address().port);
        });
      });
    })
    .then(port => {
      resolvePort(port);
    });

  return socketEgress;
}
