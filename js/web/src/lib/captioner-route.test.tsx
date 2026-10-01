import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CaptionerPage } from "../routes/captioner";
import i18n from "./i18n";

const mocks = vi.hoisted(() => ({
  benchmarkCaptionModel: vi.fn(),
  startBrowserCaptioner: vi.fn(),
  useSession: vi.fn(),
}));

vi.mock("streamplace", () => ({
  benchmarkCaptionModel: mocks.benchmarkCaptionModel,
  startBrowserCaptioner: mocks.startBrowserCaptioner,
}));
vi.mock("@/lib/session", () => ({ useSession: mocks.useSession }));
vi.mock("@tanstack/react-router", () => ({
  createFileRoute: () => (options: unknown) => options,
}));

interface StartOptions {
  signal?: AbortSignal;
}

async function flushPromises() {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}

describe("CaptionerPage lifecycle", () => {
  let container: HTMLDivElement;
  let root: Root | undefined;
  let track: { stop: () => void };
  let stream: MediaStream;
  let originalMediaDevices: PropertyDescriptor | undefined;

  beforeEach(() => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    vi.stubGlobal("crossOriginIsolated", true);
    originalMediaDevices = Object.getOwnPropertyDescriptor(
      navigator,
      "mediaDevices",
    );
    track = { stop: vi.fn() };
    stream = {
      getTracks: () => [track],
    } as unknown as MediaStream;
    Object.defineProperty(navigator, "mediaDevices", {
      configurable: true,
      value: {
        enumerateDevices: vi.fn(async () => []),
        getUserMedia: vi.fn(async () => stream),
      },
    });
    mocks.useSession.mockReturnValue({
      did: "did:plc:streamer",
      pdsAgent: {},
      signIn: vi.fn(),
    });
    mocks.benchmarkCaptionModel.mockReset();
    mocks.startBrowserCaptioner.mockReset();
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    if (root) await act(async () => root?.unmount());
    container.remove();
    if (originalMediaDevices) {
      Object.defineProperty(navigator, "mediaDevices", originalMediaDevices);
    } else {
      Reflect.deleteProperty(navigator, "mediaDevices");
    }
    vi.unstubAllGlobals();
  });

  function captionerButton(key: string): HTMLButtonElement {
    const button = Array.from(
      container.querySelectorAll<HTMLButtonElement>("button"),
    ).find((candidate) => candidate.textContent === i18n.t(key));
    if (!(button instanceof HTMLButtonElement))
      throw new Error(`Missing ${key} button`);
    return button;
  }

  async function renderPage() {
    await act(async () => {
      root?.render(<CaptionerPage />);
      await flushPromises();
    });
  }

  it("enables Stop after microphone acquisition and aborts pending startup", async () => {
    let startupSignal: AbortSignal | undefined;
    mocks.startBrowserCaptioner.mockImplementation((options: StartOptions) => {
      const { promise, reject } = Promise.withResolvers<never>();
      startupSignal = options.signal;
      options.signal?.addEventListener(
        "abort",
        () => reject(options.signal?.reason),
        { once: true },
      );
      return promise;
    });
    await renderPage();

    await act(async () => {
      captionerButton("captioner-start").click();
      await flushPromises();
    });

    const stopButton = captionerButton("captioner-stop");
    expect(stopButton.disabled).toBe(false);
    expect(startupSignal?.aborted).toBe(false);

    await act(async () => {
      stopButton.click();
      await flushPromises();
    });

    expect(startupSignal?.aborted).toBe(true);
    expect(track.stop).toHaveBeenCalled();
    expect(container.textContent).not.toContain("AbortError");
  });

  it("aborts startup and stops microphone tracks on unmount", async () => {
    let startupSignal: AbortSignal | undefined;
    mocks.startBrowserCaptioner.mockImplementation((options: StartOptions) => {
      const { promise, reject } = Promise.withResolvers<never>();
      startupSignal = options.signal;
      options.signal?.addEventListener(
        "abort",
        () => reject(options.signal?.reason),
        { once: true },
      );
      return promise;
    });
    await renderPage();
    await act(async () => {
      captionerButton("captioner-start").click();
      await flushPromises();
    });

    await act(async () => {
      root?.unmount();
      root = undefined;
      await flushPromises();
    });

    expect(startupSignal?.aborted).toBe(true);
    expect(track.stop).toHaveBeenCalled();
  });

  it("uses the session stop path after initialization succeeds", async () => {
    const stopSession = vi.fn(() => Promise.resolve());
    mocks.startBrowserCaptioner.mockResolvedValue({ stop: stopSession });
    await renderPage();

    await act(async () => {
      captionerButton("captioner-start").click();
      await flushPromises();
    });
    await act(async () => {
      captionerButton("captioner-stop").click();
      await flushPromises();
    });

    expect(stopSession).toHaveBeenCalledOnce();
    expect(track.stop).toHaveBeenCalled();
  });

  it("aborts the active benchmark on unmount without starting another model", async () => {
    let benchmarkSignal: AbortSignal | undefined;
    mocks.benchmarkCaptionModel.mockImplementation(
      (
        _nodeURL: string,
        _model: string,
        _pcm: Float32Array | undefined,
        signal: AbortSignal,
      ) => {
        const { promise, reject } = Promise.withResolvers<never>();
        benchmarkSignal = signal;
        signal.addEventListener("abort", () => reject(signal.reason), {
          once: true,
        });
        return promise;
      },
    );
    await renderPage();

    await act(async () => {
      captionerButton("captioner-measure").click();
      await flushPromises();
    });
    await act(async () => {
      root?.unmount();
      root = undefined;
      await flushPromises();
    });

    expect(benchmarkSignal?.aborted).toBe(true);
    expect(mocks.benchmarkCaptionModel).toHaveBeenCalledOnce();
  });
});
