
import munger from "./munger";

// TODO, support more than one TB, lol
const TIME_BASE = 90; // actually 90000 but JS times are in millis
const PTS_OFFSET_RESET = 500 * TIME_BASE; // if we're more than 500ms behind server time, resync.

class Syncer {
  /**
   * Create a syncer that rebases multiple mpeg streams onto the server's current time.
   * @param  {Number} options.count     Number of streams that should end up in this.streams
   * @param  {Number} options.offset    How much should I offset these streams?
   * @param  {Number} options.startTime What should I use as the start time of the server?
   */
  constructor({count, offset, startTime}) {
    this.setOffset(offset);
    this.startTime = startTime;
    this.streams = [];

    // Start these out at zero, they'll by dynamically changed later.
    this.realTimeOffset = 0;
    for (let i = 0; i < count; i++) {
      let stream = munger();
      if (i === 0) {
        stream.transformPTS = this._transformPTSFirst.bind(this);
      }
      else {
        stream.transformPTS = this._transformPTSOther.bind(this);
      }
      stream.transformDTS = this._transformDTS.bind(this);
      this.streams.push(stream);
    }
  }

  setOffset(offset) {
    this.additionalOffset = offset;
  }

  /**
   * Function to transform PTS of the first stream. It determines the offset for the rest.
   */
  _transformPTSFirst(pts) {
    const timeOffset = ((new Date()).getTime() - this.startTime) * TIME_BASE;
    const difference = Math.abs(pts + this.realTimeOffset - timeOffset);
    if (difference > PTS_OFFSET_RESET) {
      // Normalize to the server's clock
      this.realTimeOffset = timeOffset - pts;
    }
    const outputPTS = pts + this.realTimeOffset + this.additionalOffset;
    return outputPTS;
  }

  _transformPTSOther(pts) {
    return pts + this.realTimeOffset + this.additionalOffset;
  }

  _transformDTS(dts) {
    return dts + this.realTimeOffset + this.additionalOffset;
  }
}

export default function(params) {
  return new Syncer(params);
}
