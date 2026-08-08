import type { LivestreamStore } from "@streamplace/core";
import { act, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LivestreamProvider } from "./livestream-provider";

class MockWebSocket {
  onopen: (() => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;

  close() {}
}

describe("LivestreamProvider", () => {
  let container: HTMLDivElement;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    vi.stubGlobal("WebSocket", MockWebSocket);
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  });

  afterEach(() => {
    container.remove();
    vi.unstubAllGlobals();
  });

  it("creates fresh stream state when the user changes", async () => {
    const root = createRoot(container);
    let currentStore: LivestreamStore | undefined;
    const captureStore = (store: LivestreamStore): ReactNode => {
      currentStore = store;
      return null;
    };

    await act(async () => {
      root.render(
        <LivestreamProvider user="alice">{captureStore}</LivestreamProvider>,
      );
    });

    const aliceStore = currentStore!;
    act(() => aliceStore.setState({ viewers: 42 }));

    await act(async () => {
      root.render(
        <LivestreamProvider user="bob">{captureStore}</LivestreamProvider>,
      );
    });

    expect(currentStore).not.toBe(aliceStore);
    expect(currentStore?.getState().viewers).toBeNull();

    await act(async () => root.unmount());
  });
});
