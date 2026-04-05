/**
 * Web Worker that loads the MUXL WASM module and runs flat MP4 conversion.
 *
 * Communication:
 *   Main → Worker: { type: "start", fileSize: number }
 *   Worker → Main: { type: "ready", buffer: SharedArrayBuffer, readBufOffset, writeBufOffset }
 *   Worker → Main: { type: "done", tracksJson: string }
 *   Worker → Main: { type: "error", message: string }
 */

// Load WASM from public/ so Vite serves it as a static asset
// (not transformed through the JS bundler)
const wasmUrl = new URL("/wasm/muxl.js", self.location.origin);
const {
  default: init,
  convert_flat_mp4,
  read_buf_offset,
  write_buf_offset,
} = await import(/* @vite-ignore */ wasmUrl.href);

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

    // Verify WASM memory is shared (required for atomics)
    const buffer = memory.buffer;
    if (!(buffer instanceof SharedArrayBuffer)) {
      throw new Error(
        "WASM memory is not shared. The WASM binary may not have been compiled " +
        "with +atomics, or the page is not cross-origin isolated."
      );
    }

    // Tell main thread we're ready — send the SharedArrayBuffer (not the
    // Memory object, which can't be cloned). Main thread creates typed
    // views directly on this buffer.
    (self as unknown as Worker).postMessage({
      type: "ready",
      buffer,
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
