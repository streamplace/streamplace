import express from "express";
import morgan from "morgan";
import bodyParser from "body-parser";
import SP from "sp-client";
import debug from "debug";
import winston from "winston";
import RTMPInputManager from "./rtmp-input-manager";
import axios from "axios";
import parser from "xml2js-parser";

const managers = {};

const log = debug("sp:rtmp-server");
const app = express();

// Short-circuit /healthz so it doesn't log
app.use((req, res, next) => {
  if (req.url === "/healthz") {
    return res.sendStatus(200);
  }
  return next();
});

const registerInput = (req, res, app, name) => {
  SP.inputs
    .find({ streamKey: name })
    .then(([input]) => {
      if (!input) {
        log(`Unknown stream key received: ${name}`);
        const err = new Error("Unknown Stream Key");
        err.status = 404;
        return Promise.reject(err);
      }
      if (managers[name]) {
        throw new Error(`Mayday! I already have a manager for ${name}`);
      }
      managers[name] = new RTMPInputManager({
        inputId: input.id,
        rtmpUrl: `rtmp://127.0.0.1/${app}/${name}`
      });
      return res.sendStatus(200);
    })
    .catch(err => {
      winston.error(err);
      const status = err.status || 500;
      if (status === 500) {
        log("Error connecting to API server", err);
      }
      res.sendStatus(status);
    });
};

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
  registerInput(req, res, app, name);
});

app.post("/done", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/play_done", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/publish_done", (req, res, next) => {
  const manager = managers[req.body.name];
  if (!manager) {
    return winston.warn(
      `Got done for ${req.body.name} but I'm not managing it??`
    );
  }
  res.sendStatus(200);
  delete managers[req.body.name];
  manager.notify("publish_done", req.body);
});

app.post("/record_done", (req, res, next) => {
  res.sendStatus(200);
});

app.post("/update", (req, res, next) => {
  log("update");
  res.sendStatus(200);
});

SP.connect()
  .then(() => {
    app.listen(80, function() {
      log("sp-rtmp-server listening on 80");
    });
    return axios.get("http://localhost:8080");
  })
  .then(res => {
    // Currently-inoperative feature where in dev we can pick up existing
    // streams... apparently nginx-rtmp is pretty bad at reporting its current
    // stream status.
    const doc = parser.parseStringSync(res.data);
  })
  .catch(err => winston.error(err));

process.on("SIGTERM", function() {
  process.exit(0);
});
