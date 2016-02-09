
import ffmpeg from "fluent-ffmpeg";
import stream from "stream";

import SK from "../sk";
import Base from "./Base";

export default class Vertex extends Base {
  constructor({id}) {
    super();
    this.id = id;
    this.info("initializing");

    // Watch my vertex, so I can respond appropriately.
    SK.vertices.watch({id: this.id})

    .then((docs) => {
      this.doc = docs[0];
      this.info("Got initial pull.");
      this.init();
    })

    .catch((err) => {
      this.error("Error on initial pull", err);
    });
  }

  /**
   * Get a node-fluent-ffmpeg instance that does stuff we like
   */
  ffmpeg() {
    return ffmpeg()

    .on("error", (err, stdout, stderr) => {
      this.error("ffmpeg error", {err, stdout, stderr});
    })

    .on("codecData", (data) => {
      this.info("ffmpeg codecData", data);
    })

    .on("end", () => {
      this.info("ffmpeg end");
    })

    .on("progress", (data) => {
      this.info("ffmpeg progress", data);
    })

    .on("start", (command) => {
      this.info("ffmpeg start: " + command);
    });
  }
}

class RTMPInputVertex extends Vertex {
  constructor({id}) {
    super({id});
  }

  init() {
    this.inputStream = this.ffmpeg()
      .input(this.doc.rtmp.url)
      .inputFormat("flv")
      .outputOptions(["-bsf:v h264_mp4toannexb"])
      .videoCodec("libx264")
      .audioCodec("libmp3lame")
      .outputOptions([
        "-preset ultrafast",
        "-tune zerolatency",
        "-x264opts keyint=5:min-keyint="
      ])
      .outputFormat("mpegts")
      .stream();
    this.inputStream.on("data", () => {});
  }
}

Vertex.create = function(doc) {
  const {id, type} = doc;
  if (type === "RTMPInput") {
    return new RTMPInputVertex(doc);
  }
  else {
    throw new Error(`Unknown Vertex Type: ${type}`);
  }
};
