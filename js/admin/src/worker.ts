/**
 * Web Worker that loads the MUXL WASM module and runs flat MP4 conversion.
 *
 * Communication:
 *   Main → Worker: { type: "start", fileSize: number }
 *   Worker → Main: { type: "ready", memory: WebAssembly.Memory, readBufOffset, writeBufOffset }
 *   Worker → Main: { type: "done", tracksJson: string }
 *   Worker → Main: { type: "error", message: string }
 */

import init, {
  convert_flat_mp4,
  read_buf_offset,
  write_buf_offset,
} from "./wasm/muxl.js";

self.onmessage = async (e: MessageEvent) => {
  if (e.data.type !== "start") return;
  const { fileSize } = e.data as { type: "start"; fileSize: number };

  try {
    // Initialize WASM module
    const wasm = await init();
    const memory = wasm.memory;

    // Get buffer offsets
    const readOff = read_buf_offset();
    const writeOff = write_buf_offset();

    // Write file_size into the read buffer's meta field (offset + 16)
    const dv = new DataView(memory.buffer);
    dv.setBigUint64(readOff + 16, BigInt(fileSize), true);

    // Tell main thread we're ready — it needs to start serving reads
    // and draining writes before we call convert_flat_mp4 (which blocks)
    (self as unknown as Worker).postMessage({
      type: "ready",
      memory,
      readBufOffset: readOff,
      writeBufOffset: writeOff,
    });

    // Wait a tick for main thread to set up its loops
    await new Promise((r) => setTimeout(r, 0));

    // This blocks the worker thread — all I/O happens via atomics on
    // the shared WASM linear memory
    const tracksJson = convert_flat_mp4();

    (self as unknown as Worker).postMessage({
      type: "done",
      tracksJson,
    });
  } catch (e: any) {
    (self as unknown as Worker).postMessage({
      type: "error",
      message: e.message || String(e),
    });
  }
};
