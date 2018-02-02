import ffmpegCreate from "./ffmpeg";
import { PassThrough } from "stream";
import socketEgressStream from "./socket-egress-stream";
import express from "express";
import debug from "debug";

export const MANIFEST_NAME = "manifest.mpd";
export const DEFAULT_SEG_DURATION = 5000;

const log = debug("sp:dash-stream");
export default function dashStream(opts = {}) {
  const segDuration = opts.segDuration || DEFAULT_SEG_DURATION;
  const socketEgress = socketEgressStream();
  const passThrough = new PassThrough();

  const app = express();
  let ffmpeg;

  app.post("*", (req, res) => {
    const filename = req.url.slice(1);
    log(`got ${filename}`);

    // No matter what, tell ffmpeg that it's chill when we're done
    req.on("end", () => {
      res.sendStatus(200);
    });

    if (filename === MANIFEST_NAME) {
      // If it's the manifest, assemble the chunks and emit when done
      let manifest;
      req.on("data", chunk => {
        manifest += chunk.toString();
      });
      req.on("end", () => {
        passThrough.emit("manifest", manifest);
      });
    } else {
      // If it's data, pass the stream right on through
      passThrough.emit("chunk", filename, req);
    }
  });

  const listener = app.listen(0, () => {
    socketEgress.httpPort = listener.address().port;
    log(`DASH HTTP ingress listening on port ${socketEgress.httpPort}`);

    ffmpeg = ffmpegCreate()
      .input(`unix://${socketEgress.path}`)
      .inputFormat("mpegts")
      .videoCodec("copy")
      .audioCodec("copy")
      .inputOptions(["-probesize 60000000", "-analyzeduration 10000000"])
      .outputOptions([
        // This section from default options at https://ffmpeg.org/ffmpeg-all.html#dash-2
        "-bf 1",
        "-keyint_min 120",
        "-g 120",
        "-sc_threshold 0",
        "-b_strategy 0",
        "-ar:a:1 22050",
        "-use_timeline 1",
        "-use_template 1",
        "-window_size 3",
        // Avoids Tag [15][0][0][0] incompatible with output codec id '86018' (mp4a)
        "-tag:v avc1",
        "-tag:a mp4a",
        `-min_seg_duration ${segDuration * 1000}` // ms ==> microseconds
      ])
      .output(`http://127.0.0.1:${socketEgress.httpPort}/${MANIFEST_NAME}`)
      .outputFormat("dash");
  });

  passThrough.on("end", () => {
    listener.close(() => {
      log(`DASH HTTP ingress closed on port ${socketEgress.httpPort}`);
    });
  });

  passThrough.getPath = socketEgress.getPath.bind(socketEgress);

  socketEgress.getPath().then(() => {
    passThrough.pipe(socketEgress);
    ffmpeg.run();
  });

  return passThrough;
}
