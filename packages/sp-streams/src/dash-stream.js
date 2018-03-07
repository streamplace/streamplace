import ffmpegCreate from "./ffmpeg";
import { PassThrough } from "stream";
import socketEgressStream from "./socket-egress-stream";
import express from "express";
import debug from "debug";
import fs from "fs";

export const MANIFEST_NAME = "manifest.mpd";
export const DEFAULT_SEG_DURATION = 5000;
export const DEFAULT_WINDOW_SIZE = 3;

const log = debug("sp:dash-stream");
export default function dashStream(opts = {}) {
  const socketEgress = socketEgressStream();
  const passThrough = new PassThrough();

  passThrough.segDuration = opts.segDuration || DEFAULT_SEG_DURATION;
  passThrough.windowSize = opts.windowSize || DEFAULT_WINDOW_SIZE;

  const app = express();
  let ffmpeg;

  app.post("*", (req, res) => {
    log(req.url);
    const filename = req.url.slice(1);
    log(`got ${filename}`);

    // No matter what, tell ffmpeg that it's chill when we're done
    req.on("end", () => {
      res.sendStatus(200);
    });

    if (filename === MANIFEST_NAME) {
      // If it's the manifest, assemble the chunks and emit when done
      let manifest = "";
      req.on("data", chunk => {
        manifest += chunk.toString();
      });
      req.on("end", () => {
        // hack... this is usually accurate but I can't get ffmpeg to output it
        passThrough.emit("manifest", manifest.replace("mp4a.40", "mp4a.40.2"));
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
        // hls too!
        "-hls_playlist 1",
        // Avoids Tag [15][0][0][0] incompatible with output codec id '86018' (mp4a)
        "-tag:v avc1",
        "-tag:a mp4a",
        `-window_size ${passThrough.windowSize}`,
        `-min_seg_duration ${passThrough.segDuration * 1000}` // ms ==> microseconds
      ])
      .output(`http://127.0.0.1:${socketEgress.httpPort}/${MANIFEST_NAME}`)
      .outputFormat("dash");
    // hack hack hack, but for now we need a newer version
    const FFMPEG_UNSTABLE_PATH = "/usr/bin/ffmpeg-unstable";
    if (fs.existsSync(FFMPEG_UNSTABLE_PATH)) {
      ffmpeg.setFfmpegPath(FFMPEG_UNSTABLE_PATH);
    }
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
