/**
 * Main-thread handlers for the SharedArrayBuffer I/O protocol.
 *
 * The WASM module (running in a Worker) uses static buffers in its linear
 * memory for I/O. Since WASM linear memory is a SharedArrayBuffer when built
 * with +atomics, we can access these buffers directly from the main thread.
 *
 * Buffer layout (32-byte header + 1MB data):
 *   [0..4]   i32 status (atomic)
 *   [4..12]  u64 arg0 (read: offset, write: unused)
 *   [12..16] u32 arg1 (read: size requested)
 *   [16..24] u64 meta (read: file_size, write: total_written)
 *   [24..28] u32 resp (actual bytes in data region)
 *   [32..]   data region
 */

const STATUS_IDLE = 0;
const STATUS_REQUEST = 1;
const STATUS_RESPONSE = 2;
const STATUS_ERROR = 3;
const STATUS_DONE = 4;

const OFF_STATUS = 0; // in i32 units (byte offset 0)
const OFF_DATA = 32;

/**
 * Serve read requests from WASM. Loops until the conversion is complete.
 *
 * @param memory - WASM linear memory (SharedArrayBuffer-backed)
 * @param bufOffset - Byte offset of the read buffer (from read_buf_offset())
 * @param file - The input File to read from
 * @param onProgress - Called with bytes read so far
 */
export async function serveReads(
  memory: WebAssembly.Memory,
  bufOffset: number,
  file: File,
  onProgress?: (bytesRead: number) => void,
): Promise<void> {
  const i32 = new Int32Array(memory.buffer);
  const u8 = new Uint8Array(memory.buffer);
  const dv = new DataView(memory.buffer);
  const statusIdx = (bufOffset + 0) >> 2; // i32 index for status
  let totalRead = 0;

  console.log("[serveReads] Starting, bufOffset=%d statusIdx=%d, initial status=%d", bufOffset, statusIdx, Atomics.load(i32, statusIdx));

  while (true) {
    // Wait for WASM to post a request
    const current = Atomics.load(i32, statusIdx);
    if (current === STATUS_IDLE) {
      console.log("[serveReads] Status is IDLE, waiting...");
      const result = Atomics.waitAsync(i32, statusIdx, STATUS_IDLE);
      if (result.async) await result.value;
      console.log("[serveReads] Woke up from waitAsync");
    }

    const status = Atomics.load(i32, statusIdx);
    console.log("[serveReads] Status after wait: %d", status);
    if (status !== STATUS_REQUEST) break; // done or error

    // Read the request: offset (u64 LE at +4) and size (u32 LE at +12)
    const offset = dv.getBigUint64(bufOffset + 4, true);
    const size = dv.getUint32(bufOffset + 12, true);

    try {
      const slice = file.slice(Number(offset), Number(offset) + size);
      const buf = await slice.arrayBuffer();
      const bytes = new Uint8Array(buf);

      // Copy into data region
      u8.set(bytes, bufOffset + OFF_DATA);
      dv.setUint32(bufOffset + 24, bytes.length, true); // resp size

      totalRead += bytes.length;
      onProgress?.(totalRead);
    } catch (e) {
      Atomics.store(i32, statusIdx, STATUS_ERROR);
      Atomics.notify(i32, statusIdx);
      throw e;
    }

    // Signal response
    Atomics.store(i32, statusIdx, STATUS_RESPONSE);
    Atomics.notify(i32, statusIdx);
  }
}

/**
 * Drain write chunks from WASM. Loops until WASM signals DONE.
 *
 * @param memory - WASM linear memory (SharedArrayBuffer-backed)
 * @param bufOffset - Byte offset of the write buffer (from write_buf_offset())
 * @param onChunk - Called with each chunk of archive data
 * @returns Total bytes written
 */
export async function drainWrites(
  memory: WebAssembly.Memory,
  bufOffset: number,
  onChunk: (data: Uint8Array) => Promise<void>,
): Promise<number> {
  const i32 = new Int32Array(memory.buffer);
  const u8 = new Uint8Array(memory.buffer);
  const dv = new DataView(memory.buffer);
  const statusIdx = (bufOffset + 0) >> 2;
  let totalWritten = 0;

  while (true) {
    // Wait for WASM to post a chunk
    const current = Atomics.load(i32, statusIdx);
    if (current === STATUS_IDLE || current === STATUS_RESPONSE) {
      const result = Atomics.waitAsync(i32, statusIdx, current);
      if (result.async) await result.value;
    }

    const status = Atomics.load(i32, statusIdx);
    if (status === STATUS_DONE) break;
    if (status !== STATUS_REQUEST) break; // error or unexpected

    // Read chunk size from resp field
    const chunkSize = dv.getUint32(bufOffset + 24, true);
    if (chunkSize > 0) {
      // Copy out of WASM memory (must copy — the buffer will be reused)
      const chunk = new Uint8Array(chunkSize);
      chunk.set(
        u8.slice(bufOffset + OFF_DATA, bufOffset + OFF_DATA + chunkSize),
      );
      totalWritten += chunkSize;

      try {
        await onChunk(chunk);
      } catch (e) {
        Atomics.store(i32, statusIdx, STATUS_ERROR);
        Atomics.notify(i32, statusIdx);
        throw e;
      }
    }

    // Signal consumed
    Atomics.store(i32, statusIdx, STATUS_RESPONSE);
    Atomics.notify(i32, statusIdx);
  }

  return totalWritten;
}
