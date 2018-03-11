import { socketEgressStream, socketIngressStream } from "sp-streams";
import debug from "debug";
import ffmpeg from "sp-streams/dist/ffmpeg";
import { PassThrough, Transform } from "stream";

const log = debug("sp:rtmp-output-stream");

/**
 * { rtmpUrl }
 */
export default class ConvertVideoStream extends Transform {
  constructor() {
    super();
    this.socketEgress = new socketEgressStream();
    this.socketIngress = new socketIngressStream();
    this.socketIngress.on("data", chunk => this.push(chunk));
    // socketEgress.pipe = socketIngress.pipe.bind(socketIngress);
    const instance = ffmpeg()
      .input("anullsrc=channel_layout=stereo:sample_rate=44100")
      .inputFormat("lavfi")
      .input(`unix://${this.socketEgress.path}`)
      .videoCodec("libx264")
      .audioCodec("aac")
      .outputOptions([
        "-keyint_min 120",
        "-g 120",
        "-x264-params keyint=120:scenecut=0",
        "-tag:v avc1"
      ])
      .outputFormat("mpegts")
      // Video out
      .output(`unix://${this.socketIngress.path}`);

    Promise.all([this.socketEgress.getPath()]).then(() => {
      log("running");
      instance.run();
    });
  }

  _transform(chunk, encoding, cb) {
    this.socketEgress.write(chunk);
    cb();
  }
}
