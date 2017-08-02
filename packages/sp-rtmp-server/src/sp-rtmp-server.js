import express from "express";
import morgan from "morgan";
import bodyParser from "body-parser";
import SP from "sp-client";
import { rtmpInputStream, tcpEgressStream } from "sp-streams";
import debug from "debug";
import config from "sp-configuration";
import winston from "winston";

const POD_IP = config.require("POD_IP");

const log = debug("sp:rtmp-server");
const app = express();

// Short-circuit /healthz so it doesn't log
app.use((req, res, next) => {
  if (req.url === "/healthz") {
    return res.sendStatus(200);
  }
  return next();
});

app.use(morgan("dev"));
app.use(bodyParser.urlencoded({ extended: false }));

app.post("/connect", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/play", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/publish", (req, res, next) => {
  const { app, name } = req.body;
  let input;
  SP.inputs
    .find({ streamKey: name })
    .then(([_input]) => {
      input = _input;
      if (!input) {
        log(`Unknown stream key received: ${name}`);
        const err = new Error("Unknown Stream Key");
        err.status = 404;
        return Promise.reject(err);
      }
      res.sendStatus(200);
      const rtmpInput = rtmpInputStream({
        rtmpUrl: `rtmp://127.0.0.1/${app}/${name}`
      });
      rtmpInput.once("data", chunk => {
        log("got data");
      });
      const tcpEgress = tcpEgressStream();
      rtmpInput.pipe(tcpEgress);
      return tcpEgress.getPort();
    })
    .then(port => {
      return SP.streams.create({
        source: {
          kind: "Input",
          id: input.id
        },
        format: "mpegts",
        url: `tcp://${POD_IP}:${port}`,
        streams: [
          {
            media: "video"
          },
          {
            media: "audio"
          }
        ]
      });
    })
    .catch(err => {
      winston.error(err);
      const status = err.status || 500;
      if (status === 500) {
        log("Error connecting to API server", err);
      }
      res.sendStatus(status);
    });
});

app.post("/done", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/play_done", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/publish_done", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/record_done", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/update", (req, res, next) => {
  log("update");
  res.sendStatus(200);
});

SP.connect().then(() => {
  app.listen(80, function() {
    log("sp-rtmp-server listening on 80");
  });
});

process.on("SIGTERM", function() {
  process.exit(0);
});
