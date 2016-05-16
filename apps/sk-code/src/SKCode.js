
const LENGTH_OF_DAY = 86400000;
const ARRAY_LENGTH = 27; // 27 bits is exactly how much we need to encode 0-86400000

export default class SKCode {
  constructor(ts) {
    this.arr = new Uint8Array(ARRAY_LENGTH);
    this.cellArr = new Array(ARRAY_LENGTH / 3);
    for (let i = 0; i < ARRAY_LENGTH / 3; i+=1) {
      this.cellArr[i] = (new Uint8Array(3));
    }
    this.update(ts);
  }

  update(ts) {
    if (ts === undefined) {
      ts = Date.now() % LENGTH_OF_DAY;
    }
    this.value = ts;
    let idx = ARRAY_LENGTH - 1;
    while (idx >= 0) {
      this.arr[idx] = ts & 0b1;
      ts = ts >>> 1;
      idx -= 1;
    }
  }

  toColors() {
    let idx = ARRAY_LENGTH - 1;
    for (let i = this.cellArr.length - 1; i >= 0; i -= 1) {
      const cell = this.cellArr[i];
      cell[2] = this.arr[idx] ? 255 : 0;
      cell[1] = this.arr[idx - 1] ? 255 : 0;
      cell[0] = this.arr[idx - 2] ? 255 : 0;
      idx -= 3;
    }
    return this.cellArr;
  }
}

SKCode.fromColors = function(cells) {
  let ts = 0;
  let first = true;
  cells.forEach(function(cell) {
    cell.forEach((color) => {
      // Round everything to the top or bottom
      const result = color < 128 ? 0 : 1;
      // Don't bit-shift the first time we run
      if (!first) {
        ts = ts << 1;
      }
      first = false;
      ts = ts | result;
    });
  });
  return new SKCode(ts);
};
