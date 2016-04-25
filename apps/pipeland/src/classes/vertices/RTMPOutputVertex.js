
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class RTMPOutputVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    this.debug = true;
    this.videoInputURL = this.getUDPInput();
    this.audioInputURL = this.getUDPInput();
    SK.vertices.update(id, {
      inputs: [{
        name: "default",
        sockets: [{
          url: this.videoInputURL,
          type: "video"
        }, {
          url: this.audioInputURL,
          type: "audio"
        }]
      }]
    })
    .then(() => {
      this.init();
    })
    .catch((err) => {
      this.error(err);
    });
  }

  init() {
    try {
      this.ffmpeg = this.createffmpeg()
        .input(this.videoInputURL)
        .inputFormat("mpegts")
        // .inputFormat("ismv")
        .input(this.audioInputURL)
        .inputFormat("mpegts")
        .videoCodec("copy")
        // .audioCodec("copy")
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
