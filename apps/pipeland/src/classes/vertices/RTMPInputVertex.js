
import InputVertex from "./InputVertex";
import SK from "../../sk";
import ENV from "../../env";

export default class RTMPInputVertex extends InputVertex {
  constructor({id}) {
    super({id});
    this.streamFilters = ["nosignal"];
    this.videoOutputURL = this.transport.getOutputURL();
    this.audioOutputURL = this.transport.getOutputURL();
  }

  handleInitialPull() {
    super.handleInitialPull();
    this.vertexWithSockets = {
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
    };
    SK.vertices.update(this.doc.id, this.vertexWithSockets)
    .then((doc) => {
      if (this.doc.params.inputId) {
        return SK.inputs.findOne(this.doc.params.inputId);
      }
      return null;
    })
    .then((input) => {
      if (input !== null) {
        this.doc.params.rtmp = {url: `${ENV.RTMP_URL_INTERNAL}${input.streamKey}`};
        return SK.vertices.update(this.doc.id, {
          params: this.doc.params,
          title: input.title,
        });
      }
      return null;
    })
    .then(() => {
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
