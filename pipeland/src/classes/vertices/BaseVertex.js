
import ffmpeg from "fluent-ffmpeg";

import SK from "../../sk";
import Base from "../Base";

const getRandomArbitrary = function(min, max) {
  return Math.floor(Math.random() * (max - min) + min);
};
const randomPort = function() {
  return getRandomArbitrary(40000, 50000);
};

export default class BaseVertex extends Base {
  constructor({id, broadcast}) {
    super();
    this.id = id;
    this.broadcast = broadcast;
    this.info("initializing");

    // I dunno, retry counter or whatever?
    this.retryIntervals = [
      5,
      5,
      10,
      30,
    ].map(x => x * 1000);
    this.retryIdx = 0;

    // Watch my vertex, so I can respond appropriately.
    SK.vertices.watch({id: this.id})

    .then((docs) => {
      this.doc = docs[0];
      this.info("Got initial pull.");
      this.init();
    })

    .catch((err) => {
      this.error("Error on initial pull", err);
    });
  }

  /**
   * Get something that is hopefully a fresh UDP address.
   * @return {[type]} [description]
   */
  getUDP() {
    const ret = `udp://127.0.0.1:${randomPort()}?`;
    return ret;
  }

  retry() {
    const retryInterval = this.retryIntervals[this.retryIdx];
    this.retryIdx += 1;
    if (this.retryIdx >= this.retryIntervals.length) {
      this.retryIdx = this.retryIntervals.length - 1;
    }
    this.info(`Retrying in ${retryInterval / 1000} seconds`);
    setTimeout(() => {
      this.init();
    }, retryInterval);
  }

  /**
   * Get a node-fluent-ffmpeg instance that does stuff we like
   */
  ffmpeg() {
    let logCounter = 0;
    return ffmpeg()

    .outputOptions([

    ])

    .on("error", (err, stdout, stderr) => {
      this.error("ffmpeg error", {err: err.toString(), stdout, stderr});
      this.retry();
    })

    .on("codecData", (data) => {
      this.info("ffmpeg codecData", data);
    })

    .on("end", () => {
      this.info("ffmpeg end");
      this.retry();
    })

    .on("progress", (data) => {
      if (logCounter === 0) {
        this.info(`[${data.timemark}] ${data.currentFps}FPS ${data.currentKbps}Kbps`);
      }
      SK.vertices.update(this.id, {timemark: data.timemark});
      logCounter = (logCounter + 1) % 15;
    })

    .on("start", (command) => {
      this.info("ffmpeg start: " + command);
    });
  }
}
