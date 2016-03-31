
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class Combine2x2Vertex extends BaseVertex {
  constructor({id}) {
    super({id});
    this.outputURL = this.getUDP();
    this.topLeftInputURL = this.getUDPInput();
    this.topRightInputURL = this.getUDPInput();
    this.bottomLeftInputURL = this.getUDPInput();
    this.bottomRightInputURL = this.getUDPInput();
    SK.vertices.update(id, {
      inputs: {
        topleft: {
          socket: this.topLeftInputURL,
        },
        topright: {
          socket: this.topRightInputURL,
        },
        bottomleft: {
          socket: this.bottomLeftInputURL,
        },
        bottomright: {
          socket: this.bottomRightInputURL,
        },
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
        .input(this.topLeftInputURL)
        .input(this.topRightInputURL)
        .input(this.bottomLeftInputURL)
        .input(this.bottomRightInputURL)
        // .outputOptions([
        //   "-copyts"
        // ])
        .videoCodec("libx264")
        .outputOptions([
          "-copyts",
          "-vsync passthrough",
          "-filter_complex",
          [
            "[0:v]scale=960:540[topleft]",
            "[1:v]scale=960:540[topright]",
            "[2:v]scale=960:540[bottomleft]",
            "[3:v]scale=960:540[bottomright]",
            "[topleft][topright]hstack[top]",
            "[bottomleft][bottomright]hstack[bottom]",
            "[top][bottom]vstack[output]",
          ].join(";"),
          "-map [output]",
          "-preset fast",
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
