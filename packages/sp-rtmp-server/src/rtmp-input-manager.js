import { rtmpInputStream, tcpEgressStream } from "sp-streams";
import config from "sp-configuration";
import SP from "sp-client";
import winston from "winston";
import debug from "debug";

const POD_IP = config.require("POD_IP");
const STREAM_UPDATE_INTERVAL = 3000;
const log = debug("sp:rtmp-input-manager");

export default class RTMPInputManager {
  constructor({ inputId, rtmpUrl }) {
    this.inputId = inputId;
    this.rtmpInput = rtmpInputStream({ rtmpUrl });
    this.tcpEgress = tcpEgressStream();
    this.rtmpInput.pipe(this.tcpEgress);
    new Promise((resolve, reject) => {
      this.rtmpInput.once("data", chunk => {
        log("got data");
        resolve();
      });
    })
      .then(() => {
        return this.tcpEgress.getPort();
      })
      .then(port => {
        return SP.streams.create({
          source: {
            kind: "Input",
            id: this.inputId
          },
          timestamp: {
            time: Date.now(),
            pts: this.rtmpInput.currentPTS
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
      })
      .catch(err => {
        winston.error("Error setting up rtmp-input-manager", err);
      });
  }

  updateStream() {
    SP.streams
      .update(this.stream.id, {
        timestamp: {
          time: Date.now(),
          pts: this.rtmpInput.currentPTS
        }
      })
      .catch(err => winston.error(err));
  }

  shutdown() {
    this.rtmpInput.end();
    if (this.interval) {
      clearInterval(this.interval);
    }
    if (this.stream) {
      SP.streams.delete(this.stream.id).catch(err => {
        winston.error("Error cleaning up rtmp input stream", err);
      });
    }
  }

  notify(event, details) {
    if (event === "publish_done") {
      this.shutdown();
    }
  }
}
