// This file is compiled by make whisper-wasm and served beside the native
// bridge, independent of Metro/Vite's incompatible Worker bundling conventions.
const scope = globalThis as unknown as {
  location: Location;
  onmessage: (event: MessageEvent) => void;
  postMessage: (message: unknown) => void;
};
type Recognition = {
  language: string;
  segments: { text: string; startMs: number; endMs: number }[];
};
type WhisperModule = {
  FS: {
    writeFile(path: string, data: Uint8Array): void;
    unlink(path: string): void;
  };
  HEAPF32: Float32Array;
  load(path: string): boolean;
  audioBuffer(samples: number): number;
  transcribe(language: string, threads: number): Recognition;
};
type Word = { text: string; start: number; end: number };
let module: WhisperModule;
let language = "";
let samples: number[] = [];
let start = 0;
let lastSpeech = 0;
let lastDecode = 0;
let cueID = "";
let detectedLanguage = "en";
let threads = 1;
// The last decode's words that are not final yet, and the final words whose
// audio is still in the window (whisper hears them again).
let heard: Word[] = [];
let committed: Word[] = [];
// How long the last decode took; decoding more often than that would fall
// behind the microphone without bound.
let decodeMs = 0;

async function load(model: string) {
  const base = new URL(".", scope.location.href);
  // The generated native module only exists after the platform-specific build.
  const glue = await import(
    /* @vite-ignore */ new URL("caption-whisper.mjs", base).href
  );
  module = await glue.default({
    locateFile: (name: string) => new URL(name, base).href,
  });
  const response = await fetch(new URL(`ggml-${model}-q5_1.bin`, base));
  if (!response.ok)
    throw new Error(`Model download failed: ${response.status}`);
  module.FS.writeFile(
    "/model.bin",
    new Uint8Array(await response.arrayBuffer()),
  );
  if (!module.load("/model.bin"))
    throw new Error("Whisper could not load the model");
  module.FS.unlink("/model.bin");
}

function decode(pcm: Float32Array) {
  const pointer = module.audioBuffer(pcm.length);
  module.HEAPF32.set(pcm, pointer / 4);
  const began = performance.now();
  const result = module.transcribe(language, threads);
  decodeMs = performance.now() - began;
  return { ...result, rtf: decodeMs / (pcm.length / 16) };
}

const norm = (text: string) =>
  text.toLowerCase().replace(/[^\p{L}\p{N}']/gu, "");

// The window's words on the absolute clock (one per segment), without
// whisper's annotations of non-speech ("[BLANK_AUDIO]", "(music)").
function words(result: Recognition): Word[] {
  const out: Word[] = [];
  let annotation = false;
  for (const segment of result.segments) {
    const text = segment.text.trim();
    if (text.startsWith("[") || text.startsWith("(")) annotation = true;
    if (annotation) {
      annotation = !text.endsWith("]") && !text.endsWith(")");
      continue;
    }
    if (text)
      out.push({
        text,
        start: start + segment.startMs,
        end: start + segment.endMs,
      });
  }
  return out;
}

// Drops the committed words whisper heard again: the committed tail is
// matched by text at the occurrence nearest its time, else by time.
function uncommitted(now: Word[]): Word[] {
  const last = committed.at(-1);
  if (!last) return now;
  for (let k = Math.min(3, committed.length); k >= 1; k--) {
    const tail = committed.slice(-k).map((w) => norm(w.text));
    let best = -1;
    let near = k > 1 ? 3000 : 1000;
    for (let i = 0; i + k <= now.length; i++) {
      const d = Math.abs(now[i + k - 1].end - last.end);
      if (d < near && tail.every((t, j) => norm(now[i + j].text) === t)) {
        best = i + k;
        near = d;
      }
    }
    if (best >= 0) return now.slice(best);
  }
  return now.filter((w) => w.start >= last.end - 100);
}

function send(id: string, cue: Word[], final: boolean, rtf?: number) {
  scope.postMessage({
    type: "cue",
    id,
    text: cue.map((w) => w.text).join(" "),
    start: cue[0].start,
    end: cue[cue.length - 1].end,
    final,
    language: detectedLanguage,
    rtf,
  });
}

scope.onmessage = async (event: MessageEvent) => {
  try {
    const message = event.data;
    if (message.type === "init") {
      language = message.language;
      threads = Math.min(4, Math.max(1, message.threads));
      await load(message.model);
      // Real inference, not a CPU proxy. Same measurement is exposed to the UI
      // and smoke harness. A voiced fixture may be supplied by the smoke harness.
      const pcm = message.benchmarkPCM ?? new Float32Array(48000);
      const result = decode(pcm);
      scope.postMessage({ type: "ready", rtf: result.rtf, benchmark: result });
      return;
    }
    if (message.type === "audio") {
      const pcm: Float32Array = message.pcm;
      let energy = 0;
      for (const value of pcm) energy += value * value;
      const voiced = Math.sqrt(energy / pcm.length) > 0.008;
      if (!samples.length && !voiced) return;
      if (!samples.length) {
        start = message.time;
        lastSpeech = start;
        lastDecode = start;
        cueID = crypto.randomUUID();
      }
      samples.push(...pcm);
      const end = start + samples.length / 16;
      if (voiced) lastSpeech = end;
      const paused = end - lastSpeech >= 600;
      if (end - lastDecode < Math.max(paused ? 0 : 2000, decodeMs)) return;
      const result = decode(new Float32Array(samples));
      detectedLanguage = result.language || language || "en";
      lastDecode = end;
      const now = uncommitted(words(result));
      // Final: what two consecutive decodes agree on, words two seconds
      // clear of the window's end, and everything once the speaker pauses.
      let n = 0;
      while (
        n < now.length &&
        n < heard.length &&
        norm(now[n].text) === norm(heard[n].text)
      )
        n++;
      while (n < now.length && now[n].end < end - 2000) n++;
      if (paused) n = now.length;
      if (n) {
        send(cueID, now.slice(0, n), true, result.rtf);
        committed.push(...now.slice(0, n));
        cueID = crypto.randomUUID();
      }
      heard = now.slice(n);
      if (heard.length) send(cueID, heard, false, result.rtf);
      if (paused) {
        samples = [];
        committed = [];
        heard = [];
      } else if (end - start >= 12000) {
        // Keep the window bounded: committed audio need not be heard again.
        const keep = Math.max(start, (heard[0]?.start ?? end - 1000) - 300);
        samples = samples.slice(Math.round((keep - start) * 16));
        committed = committed.filter((w) => w.start >= keep);
        start = keep;
      }
    } else if (message.type === "flush") {
      if (samples.length) {
        const result = decode(new Float32Array(samples));
        detectedLanguage = result.language || detectedLanguage;
        const now = uncommitted(words(result));
        if (now.length) send(cueID, now, true);
        samples = [];
        committed = [];
        heard = [];
      }
      scope.postMessage({ type: "flushed" });
    }
  } catch (error) {
    scope.postMessage({ type: "error", message: String(error) });
  }
};
