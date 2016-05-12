
import AWS from "aws-sdk";
import _ from "underscore";
import leftPad from "left-pad";
import {PassThrough} from "stream";
import munger from "mpeg-munger";

import BaseVertex from "./BaseVertex";
import SK from "../../sk";
import ENV from "../../env";

// TODO: library this somewhere useful
const dateStamp = function() {
  const now = new Date();
  const date = [ now.getFullYear(), now.getMonth() + 1, now.getDate() ];
  return date.map((s) => leftPad(s, 2, 0)).join("-");
};

const timeStamp = function() {
  const now = new Date();
  const time = [ now.getHours(), now.getMinutes(), now.getSeconds(), now.getMilliseconds() ];
  return time.map((s) => leftPad(s, 2, 0)).join("-");
};

let s3Streamer;
if (ENV.AWS_ACCESS_KEY_ID && ENV.AWS_SECRET_ACCESS_KEY && ENV.AWS_USER_UPLOAD_BUCKET && ENV.AWS_USER_UPLOAD_PREFIX && ENV.AWS_USER_UPLOAD_REGION) {
  s3Streamer = new AWS.S3({
    accessKeyId: ENV.AWS_ACCESS_KEY_ID,
    secretAccessKey: ENV.AWS_SECRET_ACCESS_KEY,
    region: ENV.AWS_USER_UPLOAD_REGION,
  });
}

const CHUNK_UPLOAD_INTERVAL = 5/* min*/ * 60/* sec*/ * 1000/*ms*/;

export default class FileOutputVertex extends BaseVertex {
  constructor({id}) {
    if (!s3Streamer) {
      throw new Error("FileOutputVertex created, but missing required environment variables");
    }
    super({id});

    this.uploads = [];
    this.chunkIdx = 0;
    this.videoInputURL = this.transport.getInputURL();
    this.audioInputURL = this.transport.getInputURL();
    SK.vertices.update(id, {
      inputs: [{
        name: "default",
        sockets: [{
          url: this.videoInputURL,
          type: "video"
        }, {
          url: this.audioInputURL,
          type: "audio"
        }]
      }]
    })
    .then(() => {
      this.init();
    })
    .catch((err) => {
      this.error(err);
    });
  }

  init() {
    this.started = false;
    const date = dateStamp();
    const time = timeStamp();
    const folder = date.replace(/-/g, "/");
    const prefix = `${folder}/${date}-${time}-${this.doc.title}`;

    this.count = 0;
    this.doc.inputs.forEach((input) => {
      input.sockets.forEach((socket) => {
        const filePrefix = `${ENV.AWS_USER_UPLOAD_PREFIX}${prefix}-${input.name}-${socket.type}`;
        const transportStream = new this.transport.InputStream({url: socket.url});
        const mpegStream = munger(); // Just to make sure we cut files at 188-byte intervals.
        transportStream.pipe(mpegStream);
        mpegStream.once("readable", this.startIdempotent.bind(this));
        this.uploads.push({filePrefix, transportStream, mpegStream});
      });
    });
    this.uploadNextChunk();
  }

  /**
   * We don't want to sit around uploading empty files before we have the start of the stream. So,
   * this function is run when we first see some data from the transport stream.
   */
  startIdempotent() {
    if (!this.started) {
      this.started = true;
      this.uploadNextChunk();
    }
  }

  uploadNextChunk() {
    this.uploads.forEach((upload) => {
      let {filePrefix, transportStream, uploadStream, mpegStream, passThroughStream} = upload;

      // If we have an old upload stream, unpipe it.
      if (passThroughStream) {
        mpegStream.unpipe(passThroughStream);
        passThroughStream.end();
      }

      // S3's "managed uploads" advertise that they work like streams (they have pipe and unpipe)
      // but don't implement the full spec. Let's put a passthrough stream in front of it so we
      // can treat it like one.
      const fileName = `${filePrefix}-${leftPad(this.chunkIdx, 6, 0)}.ts`;
      this.info(`Uploading to ${fileName}`);

      upload.passThroughStream = PassThrough();
      mpegStream.pipe(upload.passThroughStream);
      upload.uploadStream = s3Streamer.upload({
        Bucket: ENV.AWS_USER_UPLOAD_BUCKET,
        Key:  fileName,
        Body: upload.passThroughStream
      });
      upload.uploadStream.on("httpUploadProgress", (data) => {
        const kb = Math.floor(data.loaded/1024);
        this.updateSelf({
          status: "ACTIVE",
          timemark: `${kb}k uploaded`
        });
      });
      upload.uploadStream.on("error", (err) => {
        this.error("Error uploading to S3", err);
      });
      upload.uploadStream.send();
    });
    this.chunkIdx += 1;
    this.chunkTimeoutHandle = setTimeout(this.uploadNextChunk.bind(this), CHUNK_UPLOAD_INTERVAL);
  }

  cleanup() {
    super.cleanup();
    if (this.chunkTimeoutHandle) {
      clearTimeout(this.chunkTimeoutHandle);
    }
    this.uploads.forEach(({transportStream, passThroughStream, mpegStream}) => {
      mpegStream.unpipe(passThroughStream);
      passThroughStream.end();
      transportStream.stop();
    });
  }
}
