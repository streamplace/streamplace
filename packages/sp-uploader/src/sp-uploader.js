import express from "express";
import winston from "winston";
import SP from "sp-client";
import { config } from "sp-client";
import morgan from "morgan";
import Busboy from "busboy";
import { fileOutputStream } from "sp-client";

const S3_ACCESS_KEY_ID = config.require("S3_ACCESS_KEY_ID");
const S3_SECRET_ACCESS_KEY = config.require("S3_SECRET_ACCESS_KEY");
const S3_BUCKET = config.require("S3_BUCKET");
const S3_HOST = config.require("S3_HOST");

const app = express();
// app.use(morgan.dev());

app.get("/healthz", (req, res) => {
  res.sendStatus(200);
});

app.options("*", function(req, res) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "POST");
  res.end();
});

app.post("/upload/:fileId", function(req, res) {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "POST");
  SP.files
    .find({
      id: req.params.fileId,
      uploadKey: req.query.uploadKey,
      state: "AWAITING_UPLOAD"
    })
    .then(([apiFile]) => {
      if (!apiFile) {
        return res.sendStatus(404);
      }
      const busboy = new Busboy({ headers: req.headers });
      let haveFile = false;
      busboy.on("file", (fieldname, file, filename, encoding, mimetype) => {
        if (haveFile === true) {
          res.status(400);
          res.send("You may only upload one file at a time");
          return res.end();
        }
        haveFile = true;
        const fileOutput = fileOutputStream({
          accessKeyId: S3_ACCESS_KEY_ID,
          secretAccessKey: S3_SECRET_ACCESS_KEY,
          bucket: S3_BUCKET,
          host: S3_HOST,
          prefix: `${apiFile.id}/${filename}`
        });
        file.pipe(fileOutput);
        fileOutput.on("end", () => {
          SP.files.update(apiFile.id, { state: "READY" }).catch(err => {
            winston.error("Error marking file as ready", err);
          });
        });
      });
      busboy.on("finish", () => {
        res.sendStatus(200);
      });
      req.pipe(busboy);
    });
});

SP.connect().then(() => {
  const port = process.env.PORT || 80;
  winston.info(`sp-uploader listening on port ${port}`);
  app.listen(port);
});

process.on("SIGTERM", function() {
  process.exit(0);
});
