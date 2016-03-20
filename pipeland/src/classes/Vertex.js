
import ffmpeg from "fluent-ffmpeg";
import stream from "stream";

import SK from "../sk";
import mpegts from "../mpegts-stream";
import Base from "./Base";

const getRandomArbitrary = function(min, max) {
  return Math.floor(Math.random() * (max - min) + min);
};
const randomPort = function() {
  return getRandomArbitrary(40000, 50000);
};

export default class Vertex extends Base {
  constructor({id, broadcast}) {
    super();
    this.id = id;
    this.broadcast = broadcast;
    this.info("initializing");

    // I dunno, retry counter or whatever?
    this.retryIntervals = [
      5,
      5,
      10,
      30,
    ].map(x => x * 1000);
    this.retryIdx = 0;

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
   * Get something that is hopefully a fresh UDP address.
   * @return {[type]} [description]
   */
  getUDP() {
    const ret = `udp://127.0.0.1:${randomPort()}?`;
    return ret;
  }

  retry() {
    const retryInterval = this.retryIntervals[this.retryIdx];
    this.retryIdx += 1;
    if (this.retryIdx >= this.retryIntervals.length) {
      this.retryIdx = this.retryIntervals.length - 1;
    }
    this.info(`Retrying in ${retryInterval / 1000} seconds`);
    setTimeout(() => {
      this.init();
    }, retryInterval);
  }

  /**
   * Get a node-fluent-ffmpeg instance that does stuff we like
   */
  ffmpeg() {
    let logCounter = 0;
    return ffmpeg()

    .outputOptions([

    ])

    .on("error", (err, stdout, stderr) => {
      this.error("ffmpeg error", {err: err.toString(), stdout, stderr});
      this.retry();
    })

    .on("codecData", (data) => {
      this.info("ffmpeg codecData", data);
    })

    .on("end", () => {
      this.info("ffmpeg end");
      this.retry();
    })

    .on("progress", (data) => {
      if (logCounter === 0) {
        this.info(data);
      }
      SK.vertices.update(this.id, {timemark: data.timemark});
      logCounter = (logCounter + 1) % 15;
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
    try {
      this.outputURL = this.getUDP();
      this.currentFFMpeg = this.ffmpeg()
        .input(this.doc.rtmp.url)
        .size("1280x720")
        .autopad("black")
        .inputFormat("flv")
        .outputOptions([
          // "-vsync drop",
          // "-copyts",
          "-bsf:v h264_mp4toannexb",
          // "-vf",
          // "setpts='(RTCTIME - RTCSTART) / (TB * 1000000)'"
        ])
        .videoCodec("libx264")
        .audioCodec("libmp3lame")
        .outputOptions([
          "-preset ultrafast",
          "-tune zerolatency",
          "-x264opts keyint=5:min-keyint=",
          "-pix_fmt yuv420p",
        ])
        .outputFormat("mpegts");

      this.currentFFMpeg.save(this.outputURL);
      SK.vertices.update(this.doc.id, {
        pipe: this.outputURL
      }).catch((err) => {
        this.error(err);
      });
    }
    catch (err) {
      this.error(err);
      this.retry();
    }
  }
}

class RTMPOutputVertex extends Vertex {
  constructor({id}) {
    super({id});
  }

  init() {
    try {
      this.inputURL = this.getUDP() + "reuse=1";
      SK.vertices.update(this.doc.id, {
        pipe: this.inputURL
      }).catch((err) => {
        this.error(err);
      });
      this.outputStream = this.ffmpeg()
        .input(this.inputURL)
        .inputOptions([
          // "-fflags +ignidx",
          // "-fflags +igndts",
          // "-fflags +discardcorrupt",
        ])
        .inputFormat("mpegts")
        // .inputFormat("ismv")
        .audioCodec("aac")
        .videoCodec("copy")
        // .outputOptions([
        //   "-vsync drop",
        // ])
        .outputFormat("flv")
        .videoCodec("libx264")
        .save(this.doc.rtmp.url);
        // .inputFormat("flv")
        // .outputOptions(["-bsf:v h264_mp4toannexb"])
        // .audioCodec("libmp3lame")
        // .outputOptions([
        //   "-preset ultrafast",
        //   "-tune zerolatency",
        //   "-x264opts keyint=5:min-keyint="
        // ])
        // .outputFormat("mpegts")
        // .stream();

      // We want it to start consuming data even if no arcs are listening yet. Fire away.
      // this.inputStream.resume();
    }
    catch (e) {
      this.error(e);
      this.retry();
    }
  }
}

Vertex.create = function(params) {
  const {id, type} = params;
  if (type === "RTMPInput") {
    return new RTMPInputVertex(params);
  }
  else if (type === "RTMPOutput") {
    return new RTMPOutputVertex(params);
  }
  else {
    throw new Error(`Unknown Vertex Type: ${type}`);
  }
};
