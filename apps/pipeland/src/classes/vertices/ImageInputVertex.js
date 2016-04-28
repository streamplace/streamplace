
import temp from "temp";
import fs from "fs";

import InputVertex from "./InputVertex";
import SK from "../../sk";

export default class ImageInputVertex extends InputVertex {
  constructor({id}) {
    super({id});
    this.rewriteStream = true;
    // this.debug = true;
    this.videoOutputURL = this.getUDPOutput();
    SK.vertices.update(id, {
      outputs: [{
        name: "default",
        sockets: [{
          url: this.videoOutputURL,
          type: "video"
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
      this.ffmpeg = this.createffmpeg()
        .input(this.doc.params.url)
        .inputFormat("image2")
        .inputOptions([
          "-loop 1",
          "-re"
        ])

        // Video out
        .output(this.videoOutputURL)
        .videoCodec("libx264")
        .outputOptions([
          "-map 0:v"
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
