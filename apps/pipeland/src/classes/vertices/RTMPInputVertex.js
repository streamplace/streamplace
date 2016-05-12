
import InputVertex from "./InputVertex";
import SK from "../../sk";

export default class RTMPInputVertex extends InputVertex {
  constructor({id}) {
    super({id});
    this.streamFilters = ["sync", "nosignal"];
    this.videoOutputURL = this.transport.getOutputURL();
    this.audioOutputURL = this.transport.getOutputURL();
    SK.vertices.update(id, {
      outputs: [{
        name: "default",
        sockets: [{
          url: this.videoOutputURL,
          type: "video"
        }, {
          url: this.audioOutputURL,
          type: "audio"
        }]
      }]
    })
    .then((doc) => {
      this.doc = doc;
      this.init();
    })
    .catch((err) => {
      this.error(err);
    });
  }

  init() {
    super.init();
    try {
      const offsetTime = (this.doc.params.offsetTime || 0) * 1000;
      this.ffmpeg = this.createffmpeg()
        .input(this.doc.params.rtmp.url)
        .inputFormat("flv")
        .outputOptions([
          "-bsf:v h264_mp4toannexb",
        ])

        // Video out
        .output(this.videoOutputURL)
        .videoCodec("copy")
        .outputOptions([
          "-map 0:v",
          "-copyts",
          "-start_at_zero"
        ])
        .outputFormat("mpegts")

        // Audio Out
        .output(this.audioOutputURL)
        .audioCodec("copy")
        .outputOptions([
          "-map 0:a",
          "-copyts",
          "-start_at_zero"
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
