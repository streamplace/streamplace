import SP from "sp-client";
import winston from "winston";
import config from "sp-configuration";
import { fileInputStream } from "sp-streams";

export default class FileStreamer {
  constructor({ broadcastId, podIp }) {
    winston.info(
      `FileStreamer booting up for ${JSON.stringify({ broadcastId, podIp })}`
    );
    this.broadcastId = broadcastId;
    this.podIp = podIp;
    this.myStreams = {};
    this.S3_ACCESS_KEY_ID = config.require("S3_ACCESS_KEY_ID");
    this.S3_SECRET_ACCESS_KEY = config.require("S3_SECRET_ACCESS_KEY");

    SP.broadcasts.watch({ id: broadcastId }).on("data", ([broadcast]) => {
      if (!broadcast) {
        winston.warn(
          `Looks like broadcast ${broadcastId} was deleted, holding...`
        );
        return;
      }
      broadcast.sources.filter(s => s.kind === "File").forEach(source => {
        if (this.myStreams[source.id]) {
          // We're already streamin' this one. Great.
          return;
        }
        this.initStream({ fileId: source.id });
      });
    });
  }

  initStream({ fileId }) {
    winston.info(`Booting up file stream for ${fileId}`);
    let fileInput;
    // Take up the slot immediately so we don't run multiple times
    this.myStreams[fileId] = true;
    return SP.files
      .findOne(fileId)
      .then(file => {
        return fileInputStream({
          accessKeyId: this.S3_ACCESS_KEY_ID,
          secretAccessKey: this.S3_SECRET_ACCESS_KEY,
          host: file.host,
          bucket: file.bucket,
          prefix: file.prefix
        });
      })
      .then(_fileInput => {
        fileInput = _fileInput;
        // fileInput.on("data", chunk => {
        //   console.log(chunk.length);
        // });
      })
      .catch(err => {
        winston.error(`Error initalizing stream ${fileId}`, err);
      });
  }
}
