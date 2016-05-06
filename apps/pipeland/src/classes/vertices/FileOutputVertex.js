
import AWS from "aws-sdk";

import BaseVertex from "./BaseVertex";
import {UDPInputStream} from "../UDPStreams";
import SK from "../../sk";
import ENV from "../../env";

// TODO: library this somewhere useful
const dateStamp = function() {
  const now = new Date();
  const date = [ now.getFullYear(), now.getMonth() + 1, now.getDate() ];
  const time = [ now.getHours(), now.getMinutes(), now.getSeconds(), now.getMilliseconds() ];
  for ( let i = 1; i < time.length; i++ ) {
    if ( time[i] < 10 ) {
      time[i] = "0" + time[i];
    }
  }
  return `${date.join("-")}-${time.join("-")}`;
};

let s3Streamer;
if (ENV.AWS_ACCESS_KEY_ID && ENV.AWS_SECRET_ACCESS_KEY && ENV.AWS_USER_UPLOAD_BUCKET && ENV.AWS_USER_UPLOAD_PREFIX && ENV.AWS_USER_UPLOAD_REGION) {
  s3Streamer = new AWS.S3({
    accessKeyId: ENV.AWS_ACCESS_KEY_ID,
    secretAccessKey: ENV.AWS_SECRET_ACCESS_KEY,
    region: ENV.AWS_USER_UPLOAD_REGION,
  });
}

export default class FileOutputVertex extends BaseVertex {
  constructor({id}) {
    if (!s3Streamer) {
      throw new Error("FileOutputVertex created, but missing required environment variables");
    }
    super({id});
    this.videoInputURL = this.getUDPInput();
    this.audioInputURL = this.getUDPInput();
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
    const prefix = `${dateStamp()}-${this.doc.title}`;
    this.udpStreams = [];
    this.doc.inputs.forEach((input) => {
      input.sockets.forEach((socket) => {
        const fileName = `${prefix}-${input.name}-${socket.type}.ts`;
        this.info(`Uploading to ${fileName}`);
        const udpStream = new UDPInputStream({url: socket.url});
        const uploadStream = s3Streamer.upload({
          Bucket: ENV.AWS_USER_UPLOAD_BUCKET,
          Key: ENV.AWS_USER_UPLOAD_PREFIX + fileName,
          Body: udpStream
        });
        this.udpStreams.push(udpStream);
        uploadStream.on("httpUploadProgress", (data) => {
          const kb = Math.floor(data.loaded/1024);
          this.updateSelf({
            status: "ACTIVE",
            timemark: `${kb}k uploaded`
          });
        });
        uploadStream.send();
      });
    });
  }

  cleanup() {
    super.cleanup();
    this.udpStreams.forEach((stream) => {
      stream.stop();
    });
  }
}
