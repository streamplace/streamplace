
import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class RTMPOutputVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    this.videoInputURL = this.transport.getInputURL();
    this.audioInputURL = this.transport.getInputURL();
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
      if (this.doc.params.outputId && this.doc.params.outputId !== "PREVIEW") {
        return SK.outputs.findOne(this.doc.params.outputId);
      }
      return null;
    })
    .then((output) => {
      if (output) {
        this.doc.params.rtmp = {url: output.url};
        return SK.vertices.update(id, {
          params: this.doc.params,
          title: output.title,
        });
      }
      else {
        return null;
      }
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
        .inputOptions([
          "-thread_queue_size 512",
        ])
        // .inputFormat("ismv")
        .input(this.audioInputURL)
        .inputFormat("mpegts")
        .inputOptions([
          "-thread_queue_size 512",
        ])

        .videoCodec("copy")
        .audioCodec("aac") // yuck. see https://trello.com/c/rAT84es3/55-rtmpoutputvertex-shouldn-t-recode-audio
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
