
/**
 * MPEG-TS packets are 188 bytes long. The first byte is always 0x47 (decimal 71). All this
 * transform stream does is ensures that data passing through us obeys those rules, so that
 * when we hot-swap one stream for another we don't corrupt the data steram.
 */

import {Transform} from "stream";

const PACKET_LENGTH = 188;
const SYNC_BYTE = 71;

export default function() {
  const stream = Transform();
  let fragment = new Buffer(0);
  stream._transform = function (chunk, enc, next) {
    // for (let i = 0; i < chunk.length; i += 188) {
    //   if (chunk[i] !== 71) {
    //     console.log("nope " + i);
    //   }
    // }

    let combined = Buffer.concat([fragment, chunk], fragment.length + chunk.length);
    let remainder = combined.length % PACKET_LENGTH;
    let outputLength = combined.length - remainder;
    let output = combined.slice(0, outputLength);
    if (output[0] !== SYNC_BYTE) {
      throw new Error("Out of sync :(");
    }
    this.push(output);
    fragment = combined.slice(outputLength, combined.length);
    next();
  };
  return stream;
}
