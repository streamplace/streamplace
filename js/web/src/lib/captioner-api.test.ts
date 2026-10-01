import {
  benchmarkCaptionModel,
  startBrowserCaptioner,
  type StreamplaceAgent,
} from "streamplace";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

let workerReady = false;
let addWorkletModule: () => Promise<void>;

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

describe("browser captioner cancellation", () => {
  beforeEach(() => {
    workerReady = false;
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
