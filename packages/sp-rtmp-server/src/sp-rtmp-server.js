import express from "express";
import morgan from "morgan";
import bodyParser from "body-parser";
import SP from "sp-client";
import { rtmpInputStream } from "sp-streams";
import debug from "debug";

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
  SP.inputs
    .find({ streamKey: name })
    .then(([input]) => {
      if (!input) {
        log(`Unknown stream key received: ${name}`);
        return res.sendStatus(404);
      }
      res.sendStatus(200);
      const stream = rtmpInputStream({
        rtmpUrl: `rtmp://127.0.0.1/${app}/${name}`
      });
      stream.once("data", chunk => {
        log("got data");
      });
    })
    .catch(err => {
      log("Error connecting to API server", err);
      res.sendStatus(500);
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
