
import {Transform} from "stream";

class MpegMunger extends Transform {
  constructor(params) {
    super(params);
  }

  _transform(chunk, enc, next) {
    this.push(chunk);
    next();
  }
}

export default function() {
  return new MpegMunger();
}
