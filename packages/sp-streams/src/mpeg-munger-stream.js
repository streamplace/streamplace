import { Transform } from "stream";

// Most of these constants taken from http://dvd.sourceforge.net/dvdinfo/pes-hdr.html

const PACKET_LENGTH = 188;
const SYNC_BYTE = 0x47; // 71 in decimal
const PES_START_CODE_1 = 0x0;
const PES_START_CODE_2 = 0x0;
const PES_START_CODE_3 = 0x1;
const MIN_LENGTH_PES_HEADER = 6; // Start Code (4) + PES Packet Length (2)
const START_PES_HEADER_SEARCH = 3;

const PES_INDICATOR_BYTE_OFFSET = 7;
const PES_FIRST_TIMESTAMP_BYTE_OFFSET = 9;
const PES_SECOND_TIMESTAMP_BYTE_OFFSET = 14;
const PES_INDICATOR_COMPARATOR = 0b11000000;
const PES_INDICATOR_RESULT_BOTH = 0b11000000;
const PES_INDICATOR_RESULT_PTS = 0b10000000;
const PES_INDICATOR_RESULT_NEITHER = 0b00000000;

const MPEGTS_PAYLOAD_UNIT_START_INDICATOR_MASK = 0b01000000;

const PES_TIMESTAMP_START_CODE_PTS_ONLY = 0b0010;
const PES_TIMESTAMP_START_CODE_PTS_BOTH = 0b0011;
const PES_TIMESTAMP_START_CODE_DTS_BOTH = 0b0001;

const STREAM_ID_AUDIO_START = 0xc0;
const STREAM_ID_AUDIO_END = 0xdf;
const STREAM_ID_VIDEO_START = 0xe0;
const STREAM_ID_VIDEO_END = 0xef;

export const streamIsVideo = streamId => {
  return streamId >= STREAM_ID_VIDEO_START && streamId <= STREAM_ID_VIDEO_END;
};

export const streamIsAudio = streamId => {
  return streamId >= STREAM_ID_AUDIO_START && streamId <= STREAM_ID_AUDIO_END;
};

// These will need to be better someday.
const warn = function(str) {
  /*eslint-disable no-console */
  console.error(str);
};

const zeroPad = function(str, len) {
  str = `${str}`;
  while (str.length < len) {
    str = `0${str}`;
  }
  return str;
};

// Debugging tool. Not a good way to preform bitwise math. Obviously.
const dumpByte = function(byte) {
  // Convert to a twos-complement string
  let str = (byte >>> 0).toString(2);
  // Zero-pad to eight
  str = zeroPad(str, 8);
  // console.log(str);
  return str;
};

export class MpegMungerStream extends Transform {
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
      this.end();
      // throw new Error("MPEGTS appears to be out of sync.");
    }
    const payload =
      chunk.readUInt8(startIdx + 1) & MPEGTS_PAYLOAD_UNIT_START_INDICATOR_MASK;
    if (payload === 0b00000000) {
      // No payload in this packet, we can leave!
      return;
    }
    let searchStartIdx = startIdx + START_PES_HEADER_SEARCH;
    const endIdx = startIdx + PACKET_LENGTH - MIN_LENGTH_PES_HEADER;
    for (let idx = searchStartIdx; idx < endIdx; idx += 1) {
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
      const isVideo =
        streamId >= STREAM_ID_VIDEO_START && streamId <= STREAM_ID_VIDEO_END;
      const isAudio =
        streamId >= STREAM_ID_AUDIO_START && streamId <= STREAM_ID_AUDIO_END;
      if (!isVideo && !isAudio) {
        idx += 3; // We can skip this whole dang sequence now
        continue;
      }
      this._rewriteHeader(chunk, idx, streamId);
      return;
    }
  }

  _rewriteHeader(chunk, startIdx, streamId) {
    const indicator = chunk.readUInt8(startIdx + PES_INDICATOR_BYTE_OFFSET);
    const result = indicator & PES_INDICATOR_COMPARATOR;
    let pts;
    let dts;
    if (result === PES_INDICATOR_RESULT_BOTH) {
      const ptsIdx = startIdx + PES_FIRST_TIMESTAMP_BYTE_OFFSET;
      const dtsIdx = startIdx + PES_SECOND_TIMESTAMP_BYTE_OFFSET;
      pts = this._readTimestamp(chunk, ptsIdx);
      dts = this._readTimestamp(chunk, dtsIdx);
      const newPTS = this.transformPTS(pts, dts);
      this.notifyPTS(newPTS);
      const newDTS = this.transformDTS(dts, pts);
      this._writeTimestamp(
        chunk,
        newPTS,
        ptsIdx,
        PES_TIMESTAMP_START_CODE_PTS_BOTH
      );
      this._writeTimestamp(
        chunk,
        newDTS,
        dtsIdx,
        PES_TIMESTAMP_START_CODE_DTS_BOTH
      );
    } else if (result === PES_INDICATOR_RESULT_PTS) {
      const ptsIdx = startIdx + PES_FIRST_TIMESTAMP_BYTE_OFFSET;
      pts = this._readTimestamp(chunk, ptsIdx);
      const newPTS = this.transformPTS(pts, null);
      this.notifyPTS(newPTS);
      this._writeTimestamp(
        chunk,
        newPTS,
        ptsIdx,
        PES_TIMESTAMP_START_CODE_PTS_ONLY
      );
    } else if (result === PES_INDICATOR_RESULT_NEITHER) {
      // This doesn't happen in my use case, so far as I can tell.
    } else {
      throw new Error("Unknown indicator result:" + dumpByte(result));
    }
    this.currentPTS = pts;
    this.emit("pts", { pts, streamId });
  }

  _readTimestamp(chunk, startIdx) {
    const raw = chunk.readUIntBE(startIdx, 5);
    let result = 0;
    // Credit: http://stackoverflow.com/questions/13606023/mpeg2-presentation-time-stamps-pts-calculation
    result = result | ((raw >> 3) & (0x0007 << 30));
    result = result | ((raw >> 2) & (0x7fff << 15));
    result = result | ((raw >> 1) & (0x7fff << 0));
    // const str = zeroPad(raw.toString(2), 40);
    // console.log(`${str.slice(0, 8)} ${str.slice(8, 24)} ${str.slice(24, 40)}`)
    return result;
  }

  _writeTimestamp(chunk, value, idx, startCode) {
    // Tricky because Node is limited to 32-bit bitwise operations. Yuck!
    let hiPart = (value >>> 30) | (startCode << 4) | 0b1;
    let midPart = (((value >>> 15) & ~(-1 << 15)) << 1) | 0b1;
    let loPart = ((value & ~(-1 << 15)) << 1) | 0b1;
    // const hiStr = zeroPad(hiPart.toString(2), 8);
    // const midStr = zeroPad(midPart.toString(2), 16);
    // const loStr = zeroPad(loPart.toString(2), 16);
    // console.log(`${hiStr} ${midStr} ${loStr}`);
    chunk.writeUIntBE(hiPart, idx, 1);
    chunk.writeUIntBE(midPart, idx + 1, 2);
    chunk.writeUIntBE(loPart, idx + 3, 2);
  }

  /**
   * Replace this function if you want to learn about the PTS when it happens
   */
  notifyPTS(pts) {}

  /**
   * Default function -- no-op
   */
  transformPTS(oldPTS, oldDTS) {
    return oldPTS;
  }

  /**
   * Default function -- no-op
   */
  transformDTS(oldDTS, oldPTS) {
    return oldDTS;
  }

  _transform(chunk, enc, next) {
    const chunkLength = chunk.length;
    let dataStart = 0;
    // Index where our data starts.
    let startIdx = 0;

    // Are we carrying around the remainder of the last chunk? If so, resolve that first.
    if (this.remainder !== null) {
      const lengthOfRemainderNeeded = PACKET_LENGTH - this.remainderLength;
      if (chunk.length < lengthOfRemainderNeeded) {
        // We didn't even get enough to copmlete a packet! Add what we got to the remainder and
        // move on.
        chunk.copy(this.remainder, this.remainderLength, 0, chunk.length);
        this.remainderLength = this.remainderLength + chunk.length;
        return next();
      } else {
        chunk.copy(
          this.remainder,
          this.remainderLength,
          0,
          lengthOfRemainderNeeded
        );
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

export default function mpegMungerStream(...args) {
  return new MpegMungerStream();
}
