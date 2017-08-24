/**
 * Hacky little script for dumping all the PTS from a file.
 */

/* eslint-disable no-console */

import mpegMungerStream from "./src/mpeg-munger-stream";
import fs from "fs";

const mpegMunger = mpegMungerStream();

// console.error(`Reading ${process.argv[2]}`);

const readStream = fs.createReadStream(process.argv[2]);

readStream.pipe(mpegMunger);

let ptsCount = 0;
const streamIds = {};
mpegMunger.on("pts", ({ streamId, pts }) => {
  if (!streamIds[streamId]) {
    streamIds[streamId] = {
      count: 0,
      pts: []
    };
  }
  streamIds[streamId].count += 1;
  streamIds[streamId].pts.push(pts);
});

mpegMunger.on("end", () => {
  console.log(JSON.stringify(streamIds, null, 2));
});

mpegMunger.resume();
