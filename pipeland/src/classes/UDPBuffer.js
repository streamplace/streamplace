/**
 * Okay, here's my first attempt at a UDP buffer. Two arrays with cooresponding indices. One
 * stores the precise time of arrival, one stores a reference to the Buffer. For now I'm just
 * going to have them increase forever and delete old entries when they go away. Eventually I'm
 * going to have to make this much much more efficient, I'm worried about performance here over
 * hours and hours... but for now this'll do just fine I hope.
 */

import EE from "events";

/**
 * Takes an object with "delay" in milliseconds.
 */
export default class UDPBuffer extends EE {
  constructor({delay}) {
    super();
    this.setDelay(delay);
    this.inputIdx = 0;
    this.outputIdx = 0;
    this.times = [];
    this.buffers = [];

    // Register myself statically, so one setInterval can hit all the buffers.
    UDPBuffer.buffers.push(this);
  }

  /**
   * Takes delay in millis.
   */
  setDelay(delay) {
    this.delayS = Math.floor(delay / 1000); // ms to s.
    this.delayNS = (delay - (this.delayS * 1000)) * 1000000; // ms to ns.
  }

  push(buffer) {
    this.times[this.inputIdx] = process.hrtime();
    this.buffers[this.inputIdx] = buffer;
    this.inputIdx += 1;
  }

  /**
   * Check if the provided diff exceeds our stored delay.
   */
  timeExceedsDelay(time) {
    const [s, ns] = process.hrtime(time);
    if (s > this.delayS) {
      return true;
    }
    else if (s === this.delayS) {
      return ns >= this.delayNS;
    }
    return false;
  }

  handleTick() {
    let packetTime = this.times[this.outputIdx];
    while (packetTime && this.timeExceedsDelay(packetTime)) {
      this.sendPacket();
      // SendPacket increments our outputIdx, so...
      packetTime = this.times[this.outputIdx];
    }
  }

  sendPacket() {
    const outputBuffer = this.buffers[this.outputIdx];
    if (!outputBuffer) {
      throw new Error("Tried to send a packet we didn't have!");
    }
    delete this.buffers[this.outputIdx];
    delete this.times[this.outputIdx];
    this.outputIdx += 1;
    this.emit("message", outputBuffer);
  }
}

UDPBuffer.TICK_RATE = 30;
UDPBuffer.buffers = [];
setInterval(function() {
  UDPBuffer.buffers.forEach(function(buffer) {
    buffer.handleTick();
  });
}, UDPBuffer.TICK_RATE);

setInterval(function() {
  UDPBuffer.buffers.forEach(function(buffer) {
    const size = buffer.inputIdx - buffer.outputIdx;
    buffer.emit("info", {size});
  });
}, UDPBuffer.TICK_RATE * 100);
