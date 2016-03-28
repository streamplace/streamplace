
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class DelayVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    this.outputURL = this.getUDP();
    this.inputURL = this.getUDP() + "reuse=1";
    SK.vertices.update(id, {
      inputs: {
        default: {
          socket: this.inputURL,
        }
      },
      outputs: {
        default: {
          socket: this.outputURL
        }
      },
    }).catch((err) => {
      this.error(err);
    });
  }

  init() {
    try {
      this.ffmpeg = this.createffmpeg()
        .input(this.inputURL)
        // .inputOptions("-itsoffset 00:00:05")
        .outputOptions([
        ])
        .videoCodec("libx264")
        .outputOptions([
          "-preset ultrafast",
          "-tune zerolatency",
          "-x264opts keyint=5:min-keyint=",
          "-pix_fmt yuv420p",
          "-filter_complex",
          `[0:v]setpts='(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)'[resize];[resize]scale=640:480[output]`,
          "-map [output]",
        ])
        .outputFormat("mpegts");

      this.ffmpeg.save(this.outputURL);
    }
    catch (err) {
      this.error(err);
      this.retry();
    }
  }
}
