
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class DelayVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    this.videoOutputURL = this.getUDP();
    this.audioOutputURL = this.getUDP();
    this.inputURL = this.getUDP() + "reuse=1";
    SK.vertices.update(id, {
      inputs: {
        default: {
          socket: this.inputURL,
        }
      },
      outputs: {
        video: {
          socket: this.videoOutputURL
        },
        audio: {
          socket: this.audioOutputURL
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
        .inputFormat("mpegts")
        // .inputOptions("-itsoffset 00:00:05")
        .outputOptions([
        ])
        .videoCodec("libx264")
        .audioCodec("libmp3lame")
        .outputOptions([
          "-preset ultrafast",
          "-tune zerolatency",
          "-x264opts keyint=5:min-keyint=",
          "-pix_fmt yuv420p",
          "-probesize 25000000",
          "-filter_complex",
          [
            // `[0:a]asetpts='(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)'[out_audio]`,
            `[0:v]setpts='(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)'[out_video]`,
          ].join(";")
        ])

        // Video output
        .output(this.videoOutputURL)
        .outputOptions([
          "-map [out_video]",
        ])
        .outputFormat("mpegts")

        // Audio output
        // .output(this.audioOutputURL)
        // .outputOptions([
        //   "-map [out_audio]",
        // ])
        // .outputFormat("mpegts");

      this.ffmpeg.run();
    }
    catch (err) {
      this.error(err);
      this.retry();
    }
  }
}
