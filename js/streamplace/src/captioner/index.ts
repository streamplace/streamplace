import { l } from "@atproto/lex";
import type { StreamplaceAgent } from "../agent.js";
import { place } from "../lexicons/index.js";
export { agree, shouldCommit } from "./agreement.js";

const revision = "4979e04f5dcaccb36057e059bbaed8a2f5288315";
export type CaptionModel = "tiny" | "base" | "small";
export type BrowserCue = {
  id: string;
  text: string;
  stable?: string;
  start: number;
  end: number;
  final: boolean;
  language: string;
  rtf?: number;
};
type CaptionerOptions = {
  nodeURL: string;
  agent: StreamplaceAgent;
  stream: MediaStream;
  model: CaptionModel;
  language: string;
  offsetMs?: number;
  onCue?: (cue: BrowserCue) => void;
  onSpeed?: (rtf: number) => void;
  onError?: (error: Error) => void;
  signal?: AbortSignal;
};

// Run in the worklet's real audio clock, average channels and resample to 16 kHz
// before transferring bounded 20 ms frames. No microphone access in the worker.
const workletSource = `
class CaptionAudio extends AudioWorkletProcessor {
  constructor() { super(); this.phase=0; this.sum=0; this.count=0; this.frame=[]; this.start=0; }
  process(inputs) {
    const channels=inputs[0];
    if (!channels?.length) return true;
    for(let i=0;i<channels[0].length;i++) {
      let mono=0; for(const channel of channels) mono+=channel[i]/channels.length;
      this.sum+=mono; this.count++; this.phase+=16000/sampleRate;
      if(this.phase>=1) {
        if(!this.frame.length) this.start=currentTime+i/sampleRate;
        this.frame.push(this.sum/this.count); this.sum=0; this.count=0; this.phase-=1;
        if(this.frame.length===320) {
          const pcm=new Float32Array(this.frame);
          this.port.postMessage({pcm,time:this.start},[pcm.buffer]); this.frame=[];
        }
      }
    }
    return true;
  }
}
registerProcessor('caption-audio',CaptionAudio);`;

function abortError(signal: AbortSignal): Error {
  return signal.reason instanceof Error
    ? signal.reason
    : new DOMException("The operation was aborted.", "AbortError");
}

export async function startBrowserCaptioner(
  options: CaptionerOptions,
): Promise<{ stop: () => Promise<void> }> {
  if (!globalThis.crossOriginIsolated || !globalThis.SharedArrayBuffer)
    throw new Error(
      "Captioning needs an isolated HTTPS page. Enable device captions and reload.",
    );
  if (!options.stream.getAudioTracks().length)
    throw new Error("The outgoing stream has no microphone/audio track.");
  if (options.signal?.aborted) throw abortError(options.signal);

  const worker = new Worker(
    new URL(`/api/captioner/${revision}/worker.js`, options.nodeURL),
    { type: "module" },
  );
  let audio: AudioContext;
  try {
    audio = new AudioContext({ sampleRate: 16000 });
  } catch (error) {
    worker.terminate();
    throw error;
  }

  let workletURL: string | undefined;
  let timer: ReturnType<typeof setInterval> | undefined;
  let source: MediaStreamAudioSourceNode | undefined;
  let processor: AudioWorkletNode | undefined;
  let stopped = false;
  let disconnected = false;
  let terminated = false;
  let closing: Promise<void> | undefined;
  let sending: Promise<void> | undefined;
  const pending = new Map<string, BrowserCue>();
  const terminateWorker = () => {
    if (terminated) return;
    terminated = true;
    worker.terminate();
  };
  const disconnectAudio = () => {
    if (disconnected) return;
    disconnected = true;
    clearInterval(timer);
    source?.disconnect();
    processor?.disconnect();
    if (processor) processor.port.onmessage = null;
  };
  const closeAudio = (): Promise<void> => {
    if (!closing) {
      try {
        closing = audio.close();
      } catch (error) {
        closing = Promise.reject(error);
      }
    }
    return closing;
  };
  const flush = async () => {
    if (sending || !pending.size) return sending;
    const batch = [...pending.values()].slice(0, 100);
    for (const cue of batch) pending.delete(cue.id);
    sending = (async () => {
      // A batch has one language: auto detection may change between utterances.
      for (const language of new Set(batch.map((cue) => cue.language))) {
        await options.agent.client.call(place.stream.caption.pushCaptions, {
          language,
          source: "auto",
          cues: batch
            .filter((cue) => cue.language === language)
            .map((cue) => ({
              id: cue.id,
              text: cue.text.slice(0, 2000),
              final: cue.final,
              startTime: l.toDatetimeString(
                new Date(cue.start + (options.offsetMs ?? 0)),
              ),
              endTime: l.toDatetimeString(
                new Date(
                  Math.max(cue.start, cue.end) + (options.offsetMs ?? 0),
                ),
              ),
            })),
        });
      }
    })().finally(() => {
      sending = undefined;
    });
    await sending;
  };

  const signal = options.signal;
  const aborted = Promise.withResolvers<never>();
  const onAbort = () => {
    if (!signal) return;
    terminateWorker();
    disconnectAudio();
    void closeAudio().catch(() => {});
    aborted.reject(abortError(signal));
  };
  const waitForAbort = <T>(promise: Promise<T>) =>
    signal ? Promise.race([promise, aborted.promise]) : promise;
  signal?.addEventListener("abort", onAbort, { once: true });

  try {
    workletURL = URL.createObjectURL(
      new Blob([workletSource], { type: "text/javascript" }),
    );
    await waitForAbort(audio.resume());
    const ready = Promise.withResolvers<void>();
    worker.onerror = (event) => ready.reject(new Error(event.message));
    worker.onmessage = (event: MessageEvent) => {
      if (event.data.type === "ready") {
        options.onSpeed?.(event.data.rtf);
        ready.resolve();
      }
      if (event.data.type === "error")
        ready.reject(new Error(event.data.message));
    };
    worker.postMessage({
      type: "init",
      model: options.model,
      language: options.language,
      threads: navigator.hardwareConcurrency || 1,
    });
    await waitForAbort(ready.promise);
    await waitForAbort(audio.audioWorklet.addModule(workletURL));
    source = audio.createMediaStreamSource(
      new MediaStream(options.stream.getAudioTracks()),
    );
    processor = new AudioWorkletNode(audio, "caption-audio");
    const wallClock = Date.now() - audio.currentTime * 1000;
    processor.port.onmessage = (event: MessageEvent) => {
      const { pcm, time } = event.data;
      worker.postMessage(
        { type: "audio", pcm, time: wallClock + time * 1000 },
        [pcm.buffer],
      );
    };
    worker.onmessage = (event: MessageEvent) => {
      if (event.data.type === "cue") {
        const cue: BrowserCue = event.data;
        pending.set(cue.id, cue);
        options.onCue?.(cue);
        if (cue.rtf !== undefined) options.onSpeed?.(cue.rtf);
      } else if (event.data.type === "error")
        options.onError?.(new Error(event.data.message));
    };
    source.connect(processor);
    // No output is written, but a connected output keeps worklets running.
    processor.connect(audio.destination);
    timer = setInterval(() => {
      void flush().catch((error: Error) => options.onError?.(error));
    }, 1000);
  } catch (error) {
    terminateWorker();
    disconnectAudio();
    await closeAudio().catch(() => {});
    throw error;
  } finally {
    signal?.removeEventListener("abort", onAbort);
    if (workletURL !== undefined) URL.revokeObjectURL(workletURL);
  }

  return {
    stop: async () => {
      if (stopped) return;
      stopped = true;
      disconnectAudio();
      try {
        await closeAudio();
        const previous = worker.onmessage;
        const flushed = Promise.withResolvers<void>();
        worker.onmessage = (event: MessageEvent) => {
          previous?.call(worker, event);
          if (event.data.type === "flushed") flushed.resolve();
          if (event.data.type === "error")
            flushed.reject(new Error(event.data.message));
        };
        worker.postMessage({ type: "flush" });
        await flushed.promise;
        await sending;
        while (pending.size) await flush();
      } finally {
        terminateWorker();
      }
    },
  };
}

export async function benchmarkCaptionModel(
  nodeURL: string,
  model: CaptionModel,
  pcm?: Float32Array,
  signal?: AbortSignal,
): Promise<number> {
  if (signal?.aborted) throw abortError(signal);

  const worker = new Worker(
    new URL(`/api/captioner/${revision}/worker.js`, nodeURL),
    { type: "module" },
  );
  let terminated = false;
  const terminate = () => {
    if (terminated) return;
    terminated = true;
    worker.terminate();
  };
  const ready = Promise.withResolvers<number>();
  const onAbort = () => {
    if (!signal) return;
    terminate();
    ready.reject(abortError(signal));
  };
  try {
    signal?.addEventListener("abort", onAbort, { once: true });
    worker.onerror = (event) => ready.reject(new Error(event.message));
    worker.onmessage = (event: MessageEvent) => {
      if (event.data.type === "ready") ready.resolve(event.data.rtf);
      if (event.data.type === "error")
        ready.reject(new Error(event.data.message));
    };
    worker.postMessage({
      type: "init",
      model,
      language: "en",
      threads: navigator.hardwareConcurrency || 1,
      benchmarkPCM: pcm,
    });
    return await ready.promise;
  } finally {
    signal?.removeEventListener("abort", onAbort);
    terminate();
  }
}
