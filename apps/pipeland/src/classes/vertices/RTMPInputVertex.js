
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class RTMPInputVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    this.videoOutputURL = this.getUDPOutput();
    this.audioOutputURL = this.getUDPOutput();
    SK.vertices.update(id, {
      outputs: {
        video: {
          socket: this.videoOutputURL,
          type: "video"
        },
        audio: {
          socket: this.audioOutputURL,
          type: "audio"
        }
      }
    }).catch((err) => {
      this.error(err);
    });
  }

  init() {
    try {
      const offsetTime = (this.doc.params.offsetTime || 0) * 1000;
      this.ffmpeg = this.createffmpeg()
        .input(this.doc.params.rtmp.url)
        .inputFormat("flv")
        .outputOptions([
          // "-vsync drop",
          // "-copyts",
          // "-start_at_zero",
          "-bsf:v h264_mp4toannexb",
          // "-vf",
          // `setpts='(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)'`
          // "setpts='print(TB)'"
        ])
        // .videoCodec("libx264")
        .videoCodec("copy")
        .outputOptions([
          // "-use_wallclock_as_timestamps 1",
          // "-mpegts_copyts 1",
          "-map 0:v",
          "-copyts",
          "-start_at_zero",
          // "-fflags +genpts",
          // "-timestamp NOW",
          // "-tune zerolatency",
          // "-x264opts keyint=5:min-keyint=",
          // "-pix_fmt yuv420p",
        ])
        .outputFormat("mpegts")
        .output(this.videoOutputURL)

        .output(this.audioOutputURL)
        .audioCodec("copy")
        .outputOptions([
          "-map 0:a",
          "-copyts",
          "-start_at_zero",
        ])
        .outputFormat("mpegts");

      this.ffmpeg.run();
    }
    catch (err) {
      this.error(err);
      this.retry();
    }
  }
}
