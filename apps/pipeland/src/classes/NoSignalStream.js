
/**
 * This dude is a transform stream that will always output video or audio no matter what. We use
 * that to generate "no signal" feeds if we don't have input yet.
 */

import {Transform} from "stream";
import munger from "mpeg-munger";
import url from "url";
import winston from "winston";

import {SERVER_START_TIME, TIME_BASE, PTS_OFFSET_RESET} from "../constants";
import NoSignalVertex from "./vertices/NoSignalVertex";
import PortManager from "./PortManager";

const TYPE_VIDEO = "video";
const TYPE_AUDIO = "audio";
const TICK_RATE = 3; // Rate (in ms) that we check to see if it's time to output stuff yet.

// Different states that this can be in
const STATE_EMPTY = Symbol("empty"); // nothing in our buffer. we're outputting "no signal" data.
const STATE_BUFFERING = Symbol("buffering"); // buffer is filling. wait until it's done.
const STATE_ACTIVE = Symbol("active"); // cool, we good

// Set up our backing Vertex
const vertex = new NoSignalVertex();

const videoPort = url.parse(vertex.videoOutputURL).port;
const videoServer = PortManager.createSocket(videoPort);
const mpegVideoStream = munger();
// videoServer.pipe(mpegVideoStream);
videoServer.on("message", function(msg) {
  mpegVideoStream.write(msg);
});

const audioPort = url.parse(vertex.audioOutputURL).port;
const audioServer = PortManager.createSocket(audioPort);
const mpegAudioStream = munger();
// audioServer.pipe(mpegAudioStream);
audioServer.on("message", function(msg) {
  mpegAudioStream.write(msg);
});

let ptsOffset = 0;
const transformPTS = function(pts) {
  const timeOffset = ((new Date()).getTime() - SERVER_START_TIME) * TIME_BASE;
  const difference = Math.abs(pts + ptsOffset - timeOffset);
  if (difference > PTS_OFFSET_RESET) {
    // Normalize to the server's clock
    ptsOffset = timeOffset - pts;
  }
  const outputPTS = pts + ptsOffset;
  return outputPTS;
};
const transformDTS = function(dts) {
  const outputDTS = dts + ptsOffset;
  return outputDTS;
};
mpegVideoStream.transformPTS = transformPTS;
mpegVideoStream.transformDTS = transformDTS;
mpegAudioStream.transformPTS = transformPTS;
mpegAudioStream.transformDTS = transformDTS;

let streamIdx = 0;

export default class NoSignalStream extends Transform {
  constructor({delay, type}) {
    super();
    this.setDelay(delay);
    this.type = type;
    this.streamIdx = streamIdx;
    streamIdx += 1;

    if (type === TYPE_VIDEO) {
      this.stream = mpegVideoStream;
    }
    else if (type === TYPE_AUDIO) {
      this.stream = mpegAudioStream;
    }
    else {
      throw new Error(`Unknown stream type: ${type}`);
    }
    // Buffer starts empty, obviously
    this.state = STATE_ACTIVE;

    this.inputIdx = 0;
    this.outputIdx = 0;
    this.times = [];
    this.buffers = [];

    this.stream.on("data", (chunk) => {
      if (this.state !== STATE_ACTIVE) {
        this.push(chunk);
      }
    });

    setInterval(this.handleTick.bind(this), TICK_RATE);
  }

  setDelay(delay) {
    this.delayS = Math.floor(delay / 1000); // ms to s.
    this.delayNS = (delay - (this.delayS * 1000)) * 1000000; // ms to ns.
  }

  setState(state) {
    if (this.state === state) {
      return;
    }
    winston.info(`NoSignal ${this.streamIdx} moving to state ${state.toString()}`);
    this.state = state;
  }

  _transform(chunk, enc, next) {
    this.times[this.inputIdx] = process.hrtime();
    this.buffers[this.inputIdx] = chunk;
    this.inputIdx += 1;
    if (this.state === STATE_EMPTY) {
      this.state = STATE_BUFFERING;
    }
    next();
  }

  sendPacket() {
    const outputBuffer = this.buffers[this.outputIdx];
    if (!outputBuffer) {
      throw new Error("Tried to send a packet we didn't have!");
    }
    delete this.buffers[this.outputIdx];
    delete this.times[this.outputIdx];
    this.outputIdx += 1;
    // winston.info(`NoSignal ${this.streamIdx} sending real data`);
    this.push(outputBuffer);
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
    if (!packetTime) { // We're empty. That sucks.
      this.setState(STATE_EMPTY);
    }
    else { // Cool, we output something and there's more in the buffer. We active.
      this.setState(STATE_ACTIVE);
    }
  }
}
