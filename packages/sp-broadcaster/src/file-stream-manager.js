import {
  fileInputStream,
  constantFpsStream,
  tcpEgressStream
} from "sp-streams";
import { config } from "sp-client";
import SP from "sp-client";
import winston from "winston";
import debug from "debug";

const POD_IP = config.require("POD_IP");
const STREAM_UPDATE_INTERVAL = 3000;
const log = debug("sp:file-stream-manager");

export default class FileStreamManager {
  constructor({ fileId }) {
    this.fileId = fileId;
    this.S3_ACCESS_KEY_ID = config.require("S3_ACCESS_KEY_ID");
    this.S3_SECRET_ACCESS_KEY = config.require("S3_SECRET_ACCESS_KEY");
    this.tcpEgress = tcpEgressStream();
    this.constantFps = constantFpsStream({ fps: 30 });
    this.constantFps.on("pts", ({ pts }) => {
      this.currentPts = pts;
    });
    this.constantFps.pipe(this.tcpEgress, { end: false });
    this.tcpEgress
      .getPort()
      .then(port => {
        return SP.streams.create({
          source: {
            kind: "File",
            id: this.fileId
          },
          timestamp: {
            time: Date.now(),
            pts: 0
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
      .then(stream => {
        this.stream = stream;
        this.interval = setInterval(() => {
          this.updateStream();
        }, STREAM_UPDATE_INTERVAL);
        this.streamFile();
      })
      .catch(err => {
        winston.error("Error setting up file-stream-manager", err);
      });
  }

  streamFile() {
    return SP.files
      .findOne(this.fileId)
      .then(file => {
        return fileInputStream({
          accessKeyId: this.S3_ACCESS_KEY_ID,
          secretAccessKey: this.S3_SECRET_ACCESS_KEY,
          host: file.host,
          bucket: file.bucket,
          prefix: file.id
        });
      })
      .then(fileInput => {
        this.fileStream = fileInput;
        winston.info(`File ${this.fileId} is looping.`);
        this.fileInput = fileInput;
        this.fileInput.pipe(this.constantFps, { end: false });
        this.fileInput.on("end", () => {
          log("fileInput ended");
          this.currentPts = 0;
          this.updateStream();
          this.streamFile();
        });
      })
      .catch(err => {
        winston.error(`Error streaming file ${this.fileId}`, err);
        throw err;
      });
  }

  updateStream() {
    if (!this.currentPts) {
      return;
    }
    SP.streams
      .update(this.stream.id, {
        timestamp: {
          time: Date.now(),
          pts: this.currentPts
        }
      })
      .catch(err => winston.error(err));
  }

  shutdown() {
    this.fileStream && this.rtmpInput.end();
    if (this.interval) {
      clearInterval(this.interval);
    }
    if (this.stream) {
      SP.streams.delete(this.stream.id).catch(err => {
        winston.error("Error cleaning up file input stream", err);
      });
    }
  }
}
