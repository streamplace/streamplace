/**
 * Web Worker that loads the MUXL WASM module and runs flat MP4 conversion.
 *
 * Communication:
 *   Main → Worker: { type: "start", fileSize: number }
 *   Worker → Main: { type: "ready", buffer: SharedArrayBuffer, readBufOffset, writeBufOffset }
 *   Worker → Main: { type: "done", tracksJson: string }
 *   Worker → Main: { type: "error", message: string }
 */

const post = (msg: unknown) =>
  (self as unknown as Worker).postMessage(msg);

self.onmessage = async (e: MessageEvent) => {
  if (e.data.type !== "start") return;
  const { fileSize } = e.data as { type: "start"; fileSize: number };

  try {
    console.log("[worker] Loading WASM module...");

    // Load WASM JS glue from public/ (not bundled by Vite)
    const wasmUrl = new URL("/wasm/muxl.js", self.location.origin);
    const mod = await import(/* @vite-ignore */ wasmUrl.href);
    const init = mod.default;
    const { convert_flat_mp4, read_buf_offset, write_buf_offset } = mod;

    console.log("[worker] Initializing WASM...");
    const wasm = await init();
    const memory: WebAssembly.Memory = wasm.memory;

    console.log("[worker] WASM initialized, memory buffer:", memory.buffer.constructor.name);

    // Get buffer offsets
    const readOff: number = read_buf_offset();
    const writeOff: number = write_buf_offset();
    console.log("[worker] Buffer offsets: read=%d write=%d", readOff, writeOff);

    // Verify WASM memory is shared (required for atomics)
    const buffer = memory.buffer;
    if (!(buffer instanceof SharedArrayBuffer)) {
      throw new Error(
        "WASM memory is not shared. The WASM binary may not have been compiled " +
          "with +atomics, or the page is not cross-origin isolated.",
      );
    }

    // Write file_size into the read buffer's meta field (offset + 16)
    const dv = new DataView(buffer);
    dv.setBigUint64(readOff + 16, BigInt(fileSize), true);

    // Tell main thread we're ready — send the SharedArrayBuffer (not the
    // Memory object, which can't be cloned). Main thread creates typed
    // views directly on this buffer.
    post({ type: "ready", buffer, readBufOffset: readOff, writeBufOffset: writeOff });
    console.log("[worker] Sent ready, waiting a tick before starting conversion...");

    // Wait a tick for main thread to set up its read/write loops
    await new Promise((r) => setTimeout(r, 50));

    // This blocks the worker thread — all I/O happens via atomics on
    // the shared WASM linear memory
    console.log("[worker] Starting convert_flat_mp4...");
    const tracksJson: string = convert_flat_mp4();
    console.log("[worker] Conversion complete");

    // Signal the read loop to stop (it's waiting on IDLE — set to DONE)
    const STATUS_DONE = 4;
    const i32 = new Int32Array(buffer);
    const readStatusIdx = readOff >> 2;
    Atomics.store(i32, readStatusIdx, STATUS_DONE);
    Atomics.notify(i32, readStatusIdx);

    post({ type: "done", tracksJson });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    console.error("[worker] Error:", msg);
    post({ type: "error", message: msg });
  }
};

console.log("[worker] Worker loaded, waiting for start message");
