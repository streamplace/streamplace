
import munger from "mpeg-munger";

import BaseVertex from "./BaseVertex";
import SK from "../../sk";
import ENV from "../../env";

export default class AutosyncVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    // Incoming Stream --> mpeg-munger --> ffmpeg --> quiet-js
    this.audioInputURL = this.transport.getInputURL();
    this.ffmpegInputURL = this.transport.getInputURL();
    this.ffmpegOutputURL = this.transport.getOutputURL();
  }

  notifyPTS(pts) {
    this.lastPTS = pts;
  }

  handleInitialPull() {
    super.handleInitialPull();
    SK.vertices.update(this.doc.id, {
      inputs: [{
        name: "default",
        sockets: [{
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
    const inputStream = new this.transport.InputStream({url: this.audioInputURL});
    const mpegStream = munger(); // Just to make sure we cut files at 188-byte intervals.
    mpegStream.transformPTS = (pts) => {
      this.notifyPTS(pts);
      return pts;
    };
    inputStream.pipe(mpegStream);
    mpegStream.on("data", () => {

    });
  }


  cleanup() {
    super.cleanup();
  }
}
