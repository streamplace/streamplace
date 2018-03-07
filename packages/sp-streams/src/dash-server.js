// maybe this shouldn't be in this package idk

import express from "express";
import debug from "debug";

const log = debug("sp:dash-server");

export default function dashServer(dashStream) {
  const app = express();
  let manifest;
  let files = {};

  app.use((req, res, next) => {
    log(`${req.method} ${req.url}`);
    res.header("Access-Control-Allow-Origin", "*");
    res.header(
      "Access-Control-Allow-Headers",
      "Origin, X-Requested-With, Content-Type, Accept"
    );
    next();
  });

  const keepFileDuration = (dashStream.windowSize + 1) * dashStream.segDuration;
  log(`keeping segments for ${keepFileDuration}`);

  dashStream.on("manifest", data => {
    manifest = data;
  });

  dashStream.on("chunk", (fileName, fileStream) => {
    let chunks = [];
    fileStream.on("data", chunk => chunks.push(chunk));
    fileStream.on("end", () => {
      files[fileName] = Buffer.concat(chunks);

      // Maintain the init streams, otherwise dump after the window
      if (fileName.startsWith("init") || fileName.endsWith("m3u8")) {
        return;
      }
      setTimeout(() => {
        log(`removing ${fileName}`);
        delete files[fileName];
      }, keepFileDuration);
    });
  });

  app.get("/manifest.mpd", (req, res) => {
    if (!manifest) {
      return res.sendStatus(404);
    }
    res.end(manifest);
  });

  app.get("*", (req, res) => {
    const fileName = req.url.slice(1);
    if (!files[fileName]) {
      return res.sendStatus(404);
    }
    res.end(files[fileName]);
  });

  return app;
}
