import { act, createElement, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { VideoCaptionsManager } from "../src/components/captions/video-captions-manager";

const boundary = vi.hoisted(() => ({
  call: vi.fn(),
  rejected: undefined as unknown,
}));
vi.mock("react-native", () => ({
  Platform: { OS: "android" },
  Linking: { openURL: vi.fn() },
  View: ({ children }: { children: ReactNode }) =>
    createElement("div", {}, children),
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key, i18n: { language: "en" } }),
}));
vi.mock("@streamplace/core", () => ({
  fetchCaptionTracks: async () => [],
  captionLanguageName: (language: string) => language,
  captionsUrl: () => "",
}));
vi.mock("../src/lib/theme/theme", () => ({
  useTheme: () => ({ theme: { spacing: { 2: 8, 3: 12 } } }),
}));
vi.mock("../src/streamplace-store", () => ({
  useStreamplaceStore: () => "https://node.example",
}));
vi.mock("../src/streamplace-store/xrpc", () => ({
  usePDSAgent: () => ({ client: { call: boundary.call } }),
}));
vi.mock("../src/components/ui/text", () => ({
  Text: ({ children }: { children: ReactNode }) =>
    createElement("span", {}, children),
}));
vi.mock("../src/components/ui/input", () => ({
  Input: ({
    value,
    onChangeText,
  }: {
    value: string;
    onChangeText: (value: string) => void;
  }) =>
    createElement(
      "button",
      { onClick: () => onChangeText("en") },
      value || "Choose English",
    ),
}));
vi.mock("../src/components/ui/button", () => ({
  Button: ({
    children,
    disabled,
    onPress,
  }: {
    children: ReactNode;
    disabled: boolean;
    onPress: () => Promise<void>;
  }) =>
    createElement(
      "button",
      {
        disabled,
        onClick: () => {
          void onPress().catch((error) => {
            boundary.rejected = error;
          });
        },
      },
      children,
    ),
}));

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  boundary.call.mockReset();
  boundary.rejected = undefined;
});

it("contains picker rejection, restores the upload button, and allows a later successful import", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.spyOn(console, "error").mockImplementation(() => {});
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  const picker = vi
    .fn()
    .mockRejectedValueOnce(new Error("picker unavailable"))
    .mockResolvedValueOnce({
      name: "speech.vtt",
      text: async () => "WEBVTT\n\n00:00.000 --> 00:01.000\nSpeech",
    });
  try {
    await act(async () => {
      root.render(
        <VideoCaptionsManager
          video="at://did:plc:owner/place.stream.video/video"
          pickFile={picker}
        />,
      );
    });
    await act(async () => {
      container.querySelector("button")!.click();
    });
    const upload = [...container.querySelectorAll("button")].find(
      (button) => button.textContent === "vod-captions-upload",
    )!;
    await act(async () => {
      upload.click();
    });
    expect(container.textContent).toContain("vod-captions-error-upload");
    expect(boundary.rejected).toBeUndefined();
    expect(upload.disabled).toBe(false);
    await act(async () => {
      upload.click();
    });
    expect(container.textContent).toContain("vod-captions-uploaded");
    expect(container.textContent).not.toContain("vod-captions-error-upload");
  } finally {
    await act(async () => root.unmount());
    container.remove();
  }
});
