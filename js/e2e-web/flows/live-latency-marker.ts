// Self-contained so the same marker implementation can run in Node and the
// browser page, without bundling test code into the application.
export function makeLatencyMarker() {
  const header = 0xa55a;
  const cellSize = 32;
  const columns = 16;
  const bits = 64;
  const x = 64;
  const y = 16;

  function checksum(id: number): number {
    let crc = 0xffff;
    for (let byte = 3; byte >= 0; byte--) {
      crc ^= ((id >>> (byte * 8)) & 255) << 8;
      for (let bit = 0; bit < 8; bit++) {
        crc = ((crc << 1) ^ (crc & 0x8000 ? 0x1021 : 0)) & 0xffff;
      }
    }
    return crc;
  }

  function encode(id: number): number[] {
    if (!Number.isInteger(id) || id < 0 || id > 0xffffffff) {
      throw new RangeError("frame ID must be an unsigned 32-bit integer");
    }
    const cells: number[] = [];
    for (const [value, width] of [
      [header, 16],
      [id, 32],
      [checksum(id), 16],
    ]) {
      for (let bit = width - 1; bit >= 0; bit--) {
        cells.push((value >>> bit) & 1);
      }
    }
    return cells;
  }

  function decode(brightness: ArrayLike<number>): number | null {
    if (brightness.length !== bits) return null;
    const values = [0, 0, 0];
    for (let bit = 0; bit < bits; bit++) {
      const value = brightness[bit];
      if (!Number.isFinite(value) || value < 0 || value > 255) return null;
      if (value > 64 && value < 192) return null;
      const field = bit < 16 ? 0 : bit < 48 ? 1 : 2;
      values[field] = ((values[field] << 1) | (value >= 192 ? 1 : 0)) >>> 0;
    }
    if (values[0] !== header || values[2] !== checksum(values[1])) return null;
    return values[1];
  }

  return { encode, decode, cellSize, columns, bits, x, y };
}
