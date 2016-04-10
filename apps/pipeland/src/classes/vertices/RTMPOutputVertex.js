
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class RTMPOutputVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    // this.debug = true;
    this.inputURL = this.getUDPInput();
    SK.vertices.update(id, {
      inputs: {
        default: {
          socket: this.inputURL,
        }
      }
    }).catch((err) => {
      this.error(err);
    });
  }

  init() {
    try {
      this.ffmpeg = this.createffmpeg()
        .input(this.inputURL)
        .inputFormat("mpegts")
        // .inputFormat("ismv")
        // .audioCodec("copy")
        .videoCodec("copy")
        .outputOptions([
          "-copyts",
          "-vsync passthrough",
          "-maxrate 1984k",
          "-bufsize 3968k"
        ])
        // .videoFilters("setpts='(RTCTIME - RTCSTART) / (TB * 1000000)'")
        .outputFormat("flv")
        .save(this.doc.params.rtmp.url);
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
