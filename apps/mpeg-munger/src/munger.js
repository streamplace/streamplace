
import {Transform} from "stream";

const PACKET_LENGTH = 188;
const SYNC_BYTE = 0x47; // 71 in decimal
const PES_START_CODE_1 = 0x0;
const PES_START_CODE_2 = 0x0;
const PES_START_CODE_3 = 0x1;
const MIN_LENGTH_PES_HEADER = 6; // Start Code (4) + PES Packet Length (2)
const START_PES_HEADER_SEARCH = 3;

const STREAM_ID_START = 0xC0;
const STREAM_ID_END = 0xEF;

// These will need to be better someday.
const warn = function(str) {
  /*eslint-disable no-console */
  console.error(str);
};

class MpegMunger extends Transform {
  constructor(params) {
    super(params);

    // 188-length buffer containing remainder data while we wait for the next chunk.
    this.remainder = null;

    // What's the length of the data in the remainder?
    this.remainderLength = null;
  }

  _rewrite(chunk, startIdx) {
    const sync = chunk.readUInt8(startIdx);
    if (sync !== SYNC_BYTE) {
      throw new Error("MPEGTS appears to be out of sync.");
    }
    let searchStartIdx = startIdx + START_PES_HEADER_SEARCH;
    const endIdx = startIdx + PACKET_LENGTH - MIN_LENGTH_PES_HEADER;
    for (let idx = searchStartIdx; idx < endIdx; idx+=1) {
      if (chunk.readUInt8(idx) !== PES_START_CODE_1) {
        continue;
      }
      if (chunk.readUInt8(idx + 1) !== PES_START_CODE_2) {
        idx += 1; // We already know next byte ain't a zero.
        continue;
      }
      if (chunk.readUInt8(idx + 2) !== PES_START_CODE_3) {
        continue;
      }
      const streamId = chunk.readUInt8(idx + 3);
      if (streamId < STREAM_ID_START || streamId > STREAM_ID_END) {
        idx += 3; // We can skip this whole dang sequence now
        continue;
      }
      this._rewriteHeader(chunk, idx);
      return;
    }
  }

  _rewriteHeader(chunk, startIdx) {

  }

  _transform(chunk, enc, next) {
    const chunkLength = chunk.length;
    let dataStart = 0;
    // Index where our data starts.
    let startIdx = 0;

    // Are we carrying around the remainder of the last chunk? If so, resolve that first.
    if (this.remainder !== null) {
      const lengthOfRemainderNeeded = PACKET_LENGTH - this.remainderLength;
      // We need to be resiliant to a brand new stream coming in -- check the sync byte.
      const postRemainderSync = chunk.readUInt8(lengthOfRemainderNeeded);
      if (postRemainderSync !== SYNC_BYTE) {
        warn("MPEG-TS stream appears to have been interrupted. Proceeding under the assumption we have a new stream.");
      }
      else {
        chunk.copy(this.remainder, this.remainderLength, 0, lengthOfRemainderNeeded);
        this._rewrite(this.remainder, 0);
        this.push(this.remainder);
        startIdx = lengthOfRemainderNeeded;
      }
      this.remainder = null;
      this.remainderLength = null;
    }

    let idx = startIdx;
    // Main loop.
    while (idx + PACKET_LENGTH <= chunkLength) {
      // If the remainder of the chunk is < 188 bytes, save it to be combined with the next
      // incoming chunk.
      this._rewrite(chunk, idx);
      idx += PACKET_LENGTH;
    }

    let endIdx = idx;
    if (endIdx < chunkLength) {
      this.remainder = new Buffer(PACKET_LENGTH);
      this.remainderLength = chunk.length - endIdx;
      let bytesCopied = chunk.copy(this.remainder, 0, endIdx);
    }
    this.push(chunk.slice(startIdx, endIdx));
    next();
  }
}

export default function() {
  return new MpegMunger();
}
