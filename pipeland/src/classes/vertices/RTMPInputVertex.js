
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class RTMPInputVertex extends BaseVertex {
  constructor({id}) {
    super({id});
  }

  init() {
    try {
      const offsetTime = (this.doc.params.offsetTime || 0) * 1000;
      this.outputURL = this.getUDP();
      this.ffmpeg = this.createffmpeg()
        .input(this.doc.params.rtmp.url)
        .inputFormat("flv")
        .outputOptions([
          // "-vsync drop",
          // "-copyts",
          "-bsf:v h264_mp4toannexb",
          "-vf",
          `setpts='(RTCTIME - RTCSTART + ${offsetTime}) / (TB * 1000000)'`
        ])
        .videoCodec("libx264")
        .audioCodec("libmp3lame")
        .outputOptions([
          "-copyts",
          "-preset ultrafast",
          "-tune zerolatency",
          "-x264opts keyint=5:min-keyint=",
          "-pix_fmt yuv420p",
        ])
        .outputFormat("mpegts");

      this.ffmpeg.save(this.outputURL);
      SK.vertices.update(this.doc.id, {
        outputs: {
          default: {
            socket: this.outputURL
          }
        }
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
