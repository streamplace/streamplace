import SP from "sp-client";
import winston from "winston";
import config from "sp-configuration";
import {
  fileInputStream,
  constantFpsStream,
  tcpEgressStream
} from "sp-streams";

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
        this.initStream({ fileId: source.id, podIp: this.podIp });
      });
    });
  }

  initStream({ fileId, podIp }) {
    winston.info(`Booting up file stream for ${fileId}`);
    let fileInput;
    let tcpPort;
    let stream;
    const tcpEgress = tcpEgressStream();
    // Take up the slot immediately so we don't run multiple times
    this.myStreams[fileId] = tcpEgress;
    return tcpEgress
      .getPort()
      .then(port => {
        return SP.streams.create({
          source: {
            kind: "File",
            id: fileId
          },
          timestamp: {
            time: Date.now(),
            pts: 0
          },
          format: "mpegts",
          url: `tcp://${podIp}:${port}`,
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
      .then(_stream => {
        stream = _stream;
        winston.info(`File ${fileId} got stream ${stream.id}`);
        return SP.files.findOne(fileId).then(file => {
          return fileInputStream({
            accessKeyId: this.S3_ACCESS_KEY_ID,
            secretAccessKey: this.S3_SECRET_ACCESS_KEY,
            host: file.host,
            bucket: file.bucket,
            prefix: file.prefix
          });
        });
      })
      .then(_fileInput => {
        fileInput = _fileInput;
        const constantFps = constantFpsStream({ fps: 30 });
        setInterval(() => {
          SP.streams
            .update(stream.id, {
              timestamp: {
                time: Date.now(),
                pts: constantFps.currentPTS
              }
            })
            .catch(err => winston.error(err));
        }, 3000);
        fileInput.pipe(constantFps);
        constantFps.pipe(tcpEgress);
      })
      .catch(err => {
        winston.error(`Error initalizing stream ${fileId}`, err);
      });
  }
}
