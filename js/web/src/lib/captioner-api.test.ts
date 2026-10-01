import { XrpcResponseError } from "@atproto/lex";
import {
  benchmarkCaptionModel,
  place,
  startBrowserCaptioner,
  type StreamplaceAgent,
} from "streamplace";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

let workerReady = false;
let addWorkletModule: () => Promise<void>;
let flushFailure = false;

class MockWorker {
  static instances: MockWorker[] = [];

  onerror: ((event: ErrorEvent) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  messages: unknown[] = [];
  terminate = vi.fn();

  constructor() {
    MockWorker.instances.push(this);
  }

  postMessage(message: unknown) {
    this.messages.push(message);
    let type: unknown;
    if (message !== null && typeof message === "object" && "type" in message)
      type = message.type;
    if (type === "init" && workerReady) {
      queueMicrotask(() =>
        this.onmessage?.(
          new MessageEvent("message", {
            data: { type: "ready", rtf: 0.5 },
          }),
        ),
      );
    }
    if (type === "flush") {
      if (flushFailure) {
        queueMicrotask(() =>
          this.onerror?.(
            new ErrorEvent("error", { message: "worker crashed" }),
          ),
        );
        return;
      }
      queueMicrotask(() =>
        this.onmessage?.(
          new MessageEvent("message", { data: { type: "flushed" } }),
        ),
      );
    }
  }
}

class MockAudioContext {
  static instances: MockAudioContext[] = [];

  audioWorklet = { addModule: vi.fn(() => addWorkletModule()) };
  close = vi.fn(() => Promise.resolve());
  createMediaStreamSource = vi.fn(
    () =>
      ({
        connect: vi.fn(),
        disconnect: vi.fn(),
      }) as unknown as MediaStreamAudioSourceNode,
  );
  currentTime = 0;
  destination = {} as AudioDestinationNode;
  resume = vi.fn(() => Promise.resolve());

  constructor() {
    MockAudioContext.instances.push(this);
  }
}

class MockAudioWorkletNode {
  static instances: MockAudioWorkletNode[] = [];

  connect = vi.fn();
  disconnect = vi.fn();
  port = { onmessage: null };

  constructor() {
    MockAudioWorkletNode.instances.push(this);
  }
}

class MockMediaStream {
  constructor(private readonly tracks: MediaStreamTrack[]) {}

  getAudioTracks() {
    return this.tracks;
  }
}

const inputStream = {
  getAudioTracks: () => [{} as MediaStreamTrack],
} as MediaStream;
const agent = {} as StreamplaceAgent;

function captionerOptions(signal?: AbortSignal) {
  return {
    nodeURL: "https://example.com",
    agent,
    stream: inputStream,
    model: "tiny" as const,
    language: "en",
    signal,
  };
}

describe("browser captioner", () => {
  beforeEach(() => {
    workerReady = false;
    flushFailure = false;
    addWorkletModule = () => Promise.resolve();
    MockWorker.instances = [];
    MockAudioContext.instances = [];
    MockAudioWorkletNode.instances = [];
    vi.stubGlobal("crossOriginIsolated", true);
    vi.stubGlobal("Worker", MockWorker);
    vi.stubGlobal("AudioContext", MockAudioContext);
    vi.stubGlobal("AudioWorkletNode", MockAudioWorkletNode);
    vi.stubGlobal("MediaStream", MockMediaStream);
    vi.stubGlobal(
      "URL",
      class extends URL {
        static createObjectURL = vi.fn(() => "blob:caption-worklet");
        static revokeObjectURL = vi.fn();
      },
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("terminates the worker and audio context when model initialization aborts", async () => {
    const controller = new AbortController();
    const result = startBrowserCaptioner(captionerOptions(controller.signal));

    await Promise.resolve();
    await Promise.resolve();
    expect(MockWorker.instances[0]?.messages).toContainEqual(
      expect.objectContaining({ type: "init" }),
    );

    controller.abort();

    await expect(result).rejects.toMatchObject({ name: "AbortError" });
    expect(MockWorker.instances[0]?.terminate).toHaveBeenCalledOnce();
    expect(MockAudioContext.instances[0]?.close).toHaveBeenCalledOnce();
  });

  it("terminates pending audio-worklet setup when startup aborts", async () => {
    workerReady = true;
    const { promise: pendingWorklet } = Promise.withResolvers<void>();
    addWorkletModule = () => pendingWorklet;
    const controller = new AbortController();
    const result = startBrowserCaptioner(captionerOptions(controller.signal));

    await vi.waitFor(() => {
      expect(
        MockAudioContext.instances[0]?.audioWorklet.addModule,
      ).toHaveBeenCalledOnce();
    });

    controller.abort();

    await expect(result).rejects.toMatchObject({ name: "AbortError" });
    expect(MockWorker.instances[0]?.terminate).toHaveBeenCalledOnce();
    expect(MockAudioContext.instances[0]?.close).toHaveBeenCalledOnce();
  });

  it("retains the normal flush-and-stop path after successful startup", async () => {
    workerReady = true;
    const session = await startBrowserCaptioner(captionerOptions());

    await session.stop();

    expect(MockWorker.instances[0]?.messages).toContainEqual({ type: "flush" });
    expect(MockWorker.instances[0]?.terminate).toHaveBeenCalledOnce();
    expect(MockAudioContext.instances[0]?.close).toHaveBeenCalledOnce();
    expect(
      MockAudioWorkletNode.instances[0]?.disconnect,
    ).toHaveBeenCalledOnce();
  });

  it("reports a runtime worker crash and rejects stop while releasing resources", async () => {
    workerReady = true;
    flushFailure = true;
    const onError = vi.fn();
    const session = await startBrowserCaptioner({
      ...captionerOptions(),
      onError,
    });
    await expect(session.stop()).rejects.toThrow("worker crashed");
    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({ message: "worker crashed" }),
    );
    expect(MockWorker.instances[0].terminate).toHaveBeenCalledOnce();
    expect(MockAudioContext.instances[0].close).toHaveBeenCalledOnce();
    expect(MockAudioWorkletNode.instances[0].disconnect).toHaveBeenCalledOnce();
  });

  it.each([
    new TypeError("Network unavailable"),
    new XrpcResponseError(
      place.stream.caption.pushCaptions.main,
      new Response(null, { status: 503 }),
      { encoding: "application/json", body: { error: "UpstreamFailure" } },
    ),
  ])(
    "sends failed speech before newly queued cues on the next attempt (%s)",
    async (error) => {
      vi.useFakeTimers();
      workerReady = true;
      const onError = vi.fn();
      const call = vi.fn().mockRejectedValueOnce(error).mockResolvedValue({});
      const session = await startBrowserCaptioner({
        ...captionerOptions(),
        agent: { client: { call } } as unknown as StreamplaceAgent,
        onError,
      });
      const worker = MockWorker.instances[0];
      const cue = {
        type: "cue",
        id: "failed",
        text: "Speech during outage",
        start: Date.now(),
        end: Date.now() + 1000,
        final: true,
        language: "en",
      };
      worker.onmessage?.(new MessageEvent("message", { data: cue }));
      await vi.advanceTimersByTimeAsync(1000);
      worker.onmessage?.(
        new MessageEvent("message", {
          data: { ...cue, id: "new", text: "Later speech" },
        }),
      );
      await vi.advanceTimersByTimeAsync(1000);
      expect(
        call.mock.calls[1][1].cues.map((sent: { text: string }) => sent.text),
      ).toEqual(["Speech during outage", "Later speech"]);
      expect(onError).toHaveBeenCalledExactlyOnceWith(error);
      await session.stop();
    },
  );

  it("bounds retries, preserves newer revisions, and does not resend successful language groups", async () => {
    vi.useFakeTimers();
    workerReady = true;
    const failedSend = Promise.withResolvers<void>();
    const call = vi
      .fn()
      .mockResolvedValueOnce({})
      .mockReturnValueOnce(failedSend.promise)
      .mockResolvedValue({});
    const onError = vi.fn();
    const session = await startBrowserCaptioner({
      ...captionerOptions(),
      agent: { client: { call } } as unknown as StreamplaceAgent,
      onError,
    });
    const worker = MockWorker.instances[0];
    const cue = {
      type: "cue",
      id: "failed",
      text: "Interim speech",
      start: Date.now(),
      end: Date.now() + 1000,
      final: false,
      language: "en",
    };
    const emit = (data: typeof cue) =>
      worker.onmessage?.(new MessageEvent("message", { data }));
    emit({ ...cue, id: "sent", language: "es", text: "Already sent" });
    emit(cue);
    await vi.advanceTimersByTimeAsync(1000);
    for (let i = 0; i < 110; i++) emit({ ...cue, id: `new-${i}` });
    emit({ ...cue, final: true, text: "Final speech" });
    failedSend.reject(new TypeError("Network unavailable"));
    await vi.advanceTimersByTimeAsync(1000);
    const retried = call.mock.calls[2][1].cues;
    expect(retried.map((sent: { id: string }) => sent.id)).toEqual([
      "failed",
      ...Array.from({ length: 99 }, (_, i) => `new-${i + 11}`),
    ]);
    expect(retried[0]).toMatchObject({ final: true, text: "Final speech" });
    await session.stop();
  });

  it("drops HTTP 400 speech without retrying or repeatedly reporting the error", async () => {
    vi.useFakeTimers();
    workerReady = true;
    const error = new XrpcResponseError(
      place.stream.caption.pushCaptions.main,
      new Response(null, { status: 400 }),
      { encoding: "application/json", body: { error: "InvalidRequest" } },
    );
    const onError = vi.fn();
    const call = vi.fn().mockRejectedValueOnce(error).mockResolvedValue({});
    const session = await startBrowserCaptioner({
      ...captionerOptions(),
      agent: { client: { call } } as unknown as StreamplaceAgent,
      onError,
    });
    const worker = MockWorker.instances[0];
    const cue = {
      type: "cue",
      id: "invalid",
      text: "Rejected speech",
      start: Date.now(),
      end: Date.now() + 1000,
      final: true,
      language: "en",
    };
    worker.onmessage?.(new MessageEvent("message", { data: cue }));
    await vi.advanceTimersByTimeAsync(3000);
    worker.onmessage?.(
      new MessageEvent("message", {
        data: { ...cue, id: "valid", text: "Valid speech" },
      }),
    );
    await vi.advanceTimersByTimeAsync(1000);
    expect(
      call.mock.calls.map((args) =>
        args[1].cues.map((sent: { text: string }) => sent.text),
      ),
    ).toEqual([["Rejected speech"], ["Valid speech"]]);
    expect(onError).toHaveBeenCalledExactlyOnceWith(error);
    await session.stop();
  });

  it("terminates and rejects an active benchmark when aborted", async () => {
    const controller = new AbortController();
    const result = benchmarkCaptionModel(
      "https://example.com",
      "tiny",
      undefined,
      controller.signal,
    );

    controller.abort();

    await expect(result).rejects.toMatchObject({ name: "AbortError" });
    expect(MockWorker.instances[0]?.terminate).toHaveBeenCalledOnce();
  });
});
