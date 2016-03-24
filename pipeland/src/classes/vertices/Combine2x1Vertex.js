
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class Combine2x1Vertex extends BaseVertex {
  constructor({id}) {
    super({id});
    this.outputURL = this.getUDP();
    this.leftInputURL = this.getUDP() + "reuse=1";
    this.rightInputURL = this.getUDP() + "reuse=1";
    SK.vertices.update(id, {
      inputs: {
        left: {
          socket: this.leftInputURL,
        },
        right: {
          socket: this.rightInputURL,
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
        .input(this.leftInputURL)
        .input(this.rightInputURL)
        // .outputOptions([
        //   "-copyts"
        // ])
        .videoCodec("libx264")
        .outputOptions([
          "-filter_complex [0:v][1:v]hstack[output]",
          "-map [output]",
          "-preset ultrafast",
          "-tune zerolatency",
          "-x264opts keyint=5:min-keyint=",
          "-pix_fmt yuv420p",
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
