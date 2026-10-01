import { agree, shouldCommit } from "./agreement.js";

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
let module: WhisperModule;
let language = "";
let samples: number[] = [];
let start = 0;
let lastSpeech = 0;
let lastDecode = 0;
let hypothesis = "";
let cueID = "";
let detectedLanguage = "en";
let threads = 1;

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
  return { ...result, rtf: (performance.now() - began) / (pcm.length / 16) };
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
      const final = shouldCommit(end - lastSpeech, end - start);
      if (!final && end - lastDecode < 2000) return;
      const result = decode(new Float32Array(samples));
      detectedLanguage = result.language || language || "en";
      const text = result.segments
        .map((segment: { text: string }) => segment.text)
        .join(" ")
        .trim();
      const agreed = agree(hypothesis, text);
      hypothesis = agreed.text;
      lastDecode = end;
      if (text)
        scope.postMessage({
          type: "cue",
          id: cueID,
          text,
          stable: agreed.stable,
          start,
          end: lastSpeech,
          final,
          language: detectedLanguage,
          rtf: result.rtf,
        });
      if (final) {
        samples = [];
        hypothesis = "";
        lastDecode = end;
      }
    } else if (message.type === "flush") {
      if (samples.length) {
        const result = decode(new Float32Array(samples));
        const text = result.segments
          .map((segment: { text: string }) => segment.text)
          .join(" ")
          .trim();
        if (text)
          scope.postMessage({
            type: "cue",
            id: cueID,
            text,
            start,
            end: lastSpeech,
            final: true,
            language: result.language || detectedLanguage,
          });
        samples = [];
      }
      scope.postMessage({ type: "flushed" });
    }
  } catch (error) {
    scope.postMessage({ type: "error", message: String(error) });
  }
};
