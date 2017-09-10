/**
 * Basic implementation of a buffered stack. Outputs thing zero unless we haven't heard from it in
 * a few seconds, then it goes to thing 1, etc...
 *
 * We'll eventually want this to be aware of things like frames and PTS but this here is enough to
 * get started.
 */
import ptsNormalizerStream from "./pts-normalizer-stream";
import tcpIngressStream from "./tcp-ingress-stream";
import debug from "debug";

const log = debug("sp:buffer-stack");

export class BufferStack {
  constructor({ delay }) {
    this.delay = delay;
    this.outputStream = new ptsNormalizerStream();
    this.sources = {};
    this.sourceOrder = [];
    this.currentStream = null;
    this.interval = setInterval(this.reconcile.bind(this), 250);
  }

  cleanup() {
    clearInterval(this.interval);
  }

  /**
   * Upsert this stack a stream.
   * @param {String} id
   * @param {String} url
   */
  stream(id, url) {
    debug(`Connecting to ${url} for source ${id}`);
    const source = {
      stream: tcpIngressStream({ url }),
      time: 0
    };
    source.stream.on("data", () => {
      source.time = Date.now();
    });
    source.stream.once("data", () => {
      this.sources[id] = source;
      this.reconcile();
    });
  }

  /**
   * Run frequently. Based on everything that you know, pipe and unpipe the appropriate streams.
   */
  reconcile() {
    const now = Date.now();
    for (let i = 0; i < this.sourceOrder.length; i++) {
      const id = this.sourceOrder[i];
      if (!this.sources[id]) {
        // We got nothin' for this one yet.
        continue;
      }
      const source = this.sources[id];
      if (now - source.time > this.delay) {
        log(`Rejecting ${id}, no data for ${now - source.time}ms`);
        // We haven't heard from this one in like 10000 years
        continue;
      }
      // Found the correct output stream, hooray!
      if (this.currentStream === source.stream) {
        // We were already corrrect. Carry on.
        return;
      }
      log(`Switching output to source ${id}`);
      if (this.currentStream !== null) {
        this.currentStream.unpipe(this.outputStream);
      }
      this.outputStream.renormalize();
      source.stream.pipe(this.outputStream);
      this.currentStream = source.stream;
      return;
    }
    // If we made it here, we got nothin'. Boo.
    if (this.currentStream) {
      log(`Clearing output, no data`);
      this.currentStream.unpipe(this.outputStream);
      this.currentStream = null;
    }
  }

  /**
   * Order or reorder this stack. Just go ahead and pass me an array of ids.
   * @param {String[]} ids
   * @param {Object} sources[].stream
   * @param {string} sources[].id
   */
  order(ids) {
    log(`New order: ${JSON.stringify(ids)}`);
    this.sourceOrder = ids;
    this.reconcile();
  }

  pipe(dest) {
    this.outputStream.pipe(dest);
  }

  unpipe(dest) {
    this.outputStream.unpipe(dest);
  }
}

export default function(params) {
  return new BufferStack(params);
}
