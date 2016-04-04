
import fluentffmpeg from "fluent-ffmpeg";
import {parse, quote} from "shell-quote";

import M from "../MagicFilters";
import SK from "../../sk";
import Base from "../Base";

const getRandomArbitrary = function(min, max) {
  return Math.floor(Math.random() * (max - min) + min);
};
const randomPort = function() {
  return getRandomArbitrary(40000, 50000);
};

// This is used as the "00:00:00" point for keeping all streams in sync.
const SERVER_START_TIME = (new Date()).getTime() * 1000;

export default class BaseVertex extends Base {
  constructor({id, broadcast}) {
    super();
    this.id = id;
    this.broadcast = broadcast;
    this.info("initializing");
    this.debug = false;

    this.SERVER_START_TIME = SERVER_START_TIME;

    // I dunno, retry counter or whatever?
    this.retryIntervals = [
      5,
      5,
      10,
      30,
    ].map(x => x * 1000);
    this.retryIdx = 0;

    // Watch my vertex, so I can respond appropriately.
    this.vertexHandle = SK.vertices.watch({id: this.id})

    .then(([doc]) => {
      this.doc = doc;
      SK.vertices.update(this.id, {
        status: "WAITING",
        timemark: ""
      });
      this.info("Got initial pull.");
      this.handleInitialPull(doc);
    })

    .on("updated", ([doc]) => {
      this.doc = doc;
    })

    .catch((err) => {
      this.error("Error on initial pull", err);
    })

    .on("deleted", () => {
      this.cleanup();
    });
  }

  handleInitialPull(doc) {
    this.init();
  }

  /**
   * Get something that is hopefully a fresh UDP address.
   */
  getUDP() {
    const ret = `udp://127.0.0.1:${randomPort()}?`;
    return ret;
  }

  getUDPInput() {
    return this.getUDP() + "reuse=1&fifo_size=1000000&overrun_nonfatal=1";
  }

  cleanup() {
    this.deleted = true;
    if (this.ffmpeg) {
      this.ffmpeg.kill("SIGKILL");
    }
    if (this.vertexHandle) {
      this.vertexHandle.stop();
    }
    this.info("Cleaning up");
  }

  retry() {
    const retryInterval = this.retryIntervals[this.retryIdx];
    this.retryIdx += 1;
    if (this.retryIdx >= this.retryIntervals.length) {
      this.retryIdx = this.retryIntervals.length - 1;
    }
    this.info(`Retrying in ${retryInterval / 1000} seconds`);
    this.timeoutHandle = setTimeout(() => {
      delete this.timeoutHandle;
      this.init();
    }, retryInterval);
  }

  /**
   * Get a node-fluent-ffmpeg instance that does stuff we like. Also, hey. There is no good way to
   * camelcase ffmpeg. I'm just going to leave this method all lower case. Deal.
   * https://twitter.com/elimallon/status/713086603850178561
   */
  createffmpeg() {
    let logCounter = 0;
    const ffmpeg = fluentffmpeg()

    .outputOptions([

    ])

    .on("error", (err, stdout, stderr) => {
      if (this.deleted) {
        return;
      }
      this.error("ffmpeg error", {err: err.toString(), stdout, stderr});
      this.retry();
    })

    .on("codecData", (data) => {
      this.info("ffmpeg codecData", data);
      SK.vertices.update(this.id, {
        status: "CODEC",
      });
    })

    .on("end", () => {
      this.info("ffmpeg end");
      this.retry();
    })

    .on("progress", (data) => {
      if (logCounter === 0) {
        this.info(`[${data.timemark}] ${data.currentFps}FPS ${data.currentKbps}Kbps`);
      }
      SK.vertices.update(this.id, {
        status: "ACTIVE",
        timemark: data.timemark
      });
      logCounter = (logCounter + 1) % 15;
    })

    .on("start", (command) => {
      const sanitizedCommand = command;
      this.info("ffmpeg start: " + sanitizedCommand);
      if (this.debug === true) {
        ffmpeg.ffmpegProc.stdout.on("data", (data) => {
          this.info(data.toString());
        });
        ffmpeg.ffmpegProc.stderr.on("data", (data) => {
          this.info(data.toString());
        });
      }
    });
    return ffmpeg;
  }
}
