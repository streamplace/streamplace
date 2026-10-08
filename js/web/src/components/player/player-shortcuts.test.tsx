import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FullscreenProvider } from "../../contexts/fullscreen-context";
import "../../lib/i18n";
import { PlayerControls, type PlayerControlsProps } from "./player-controls";

describe("player shortcuts", () => {
  let container: HTMLDivElement;
  let video: HTMLVideoElement;
  let root: Root;
  let paused: boolean;
  let props: PlayerControlsProps;

  beforeEach(async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    video = document.createElement("video");
    document.body.append(container, video);
    paused = true;
    // jsdom has no media decoder. Keep playback state and events together.
    Object.defineProperties(video, {
      duration: { configurable: true, value: 100 },
      paused: { get: () => paused },
    });
    vi.spyOn(video, "play").mockImplementation(async () => {
      paused = false;
      video.dispatchEvent(new Event("play"));
    });
    vi.spyOn(video, "pause").mockImplementation(() => {
      paused = true;
      video.dispatchEvent(new Event("pause"));
    });
    video.currentTime = 50;
    props = {
      videoRef: { current: video },
      containerRef: { current: container },
      isLive: false,
      showControls: true,
      bigPlay: false,
      qualities: [{ index: 0, label: "1080p", frameRate: 60 }],
      currentQuality: 0,
      onQualityChange: () => {},
      useWebRTC: false,
      onUseWebRTCChange: () => {},
      showStats: false,
      onShowStatsChange: () => {},
      showDanmu: false,
      onShowDanmuChange: () => {},
    };
    root = createRoot(container);
    await render();
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    video.remove();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  async function render() {
    await act(async () => {
      root.render(
        <FullscreenProvider>
          <PlayerControls {...props} />
        </FullscreenProvider>,
      );
    });
  }

  async function press(
    key: string,
    target: EventTarget = document.body,
    options: KeyboardEventInit = {},
  ) {
    const event = new KeyboardEvent("keydown", {
      key,
      bubbles: true,
      cancelable: true,
      ...options,
    });
    await act(async () => target.dispatchEvent(event));
    return event;
  }

  it("toggles playback with k and space", async () => {
    await press("k");
    expect(video.paused).toBe(false);
    await press(" ");
    expect(video.paused).toBe(true);
  });

  it("steps paused frames using the selected video's frame rate", async () => {
    await press(".");
    expect(video.currentTime).toBeCloseTo(50 + 1 / 60);
    await press(",");
    expect(video.currentTime).toBeCloseTo(50);
    await press(",");
    expect(video.currentTime).toBeCloseTo(50 - 1 / 60);
  });

  it("does not step frames during playback", async () => {
    await act(async () => video.play());
    await press(".");
    await press(",");
    expect(video.currentTime).toBe(50);
    expect(video.paused).toBe(false);
  });

  it("uses a 30 fps fallback when frame metadata is missing", async () => {
    props.qualities = [];
    await render();
    await press(".");
    expect(video.currentTime).toBeCloseTo(50 + 1 / 30);
  });

  it("keeps frame stepping within the recording", async () => {
    video.currentTime = 0;
    await press(",");
    expect(video.currentTime).toBe(0);
    video.currentTime = 100 - 1 / 120;
    await press(".");
    expect(video.currentTime).toBe(100);
  });

  it("adjusts speed in quarter steps without stepping frames", async () => {
    await press(">", document.body, { shiftKey: true });
    expect(video.playbackRate).toBe(1.25);
    await press("<", document.body, { shiftKey: true });
    expect(video.playbackRate).toBe(1);
    expect(video.currentTime).toBe(50);
  });

  it("bounds speed between 0.25x and 2x", async () => {
    await act(async () => {
      video.playbackRate = 2;
    });
    await press(">");
    expect(video.playbackRate).toBe(2);
    await act(async () => {
      video.playbackRate = 0.25;
    });
    await press("<");
    expect(video.playbackRate).toBe(0.25);
  });

  it("seeks by five or ten seconds and clamps at the ends", async () => {
    await press("ArrowLeft");
    expect(video.currentTime).toBe(45);
    await press("l");
    expect(video.currentTime).toBe(55);
    await press("ArrowRight");
    expect(video.currentTime).toBe(60);
    await press("j");
    expect(video.currentTime).toBe(50);
    video.currentTime = 2;
    await press("j");
    expect(video.currentTime).toBe(0);
    video.currentTime = 98;
    await press("l");
    expect(video.currentTime).toBe(100);
  });

  it("jumps to percentages and the beginning or end", async () => {
    await press("9");
    expect(video.currentTime).toBe(90);
    await press("0");
    expect(video.currentTime).toBe(0);
    await press("End");
    expect(video.currentTime).toBe(100);
    await press("Home");
    expect(video.currentTime).toBe(0);
  });

  it("changes volume by five percent, unmutes, and respects the limits", async () => {
    await act(async () => {
      video.volume = 0.5;
      video.muted = true;
    });
    await press("ArrowUp");
    expect(video.volume).toBeCloseTo(0.55);
    expect(video.muted).toBe(false);
    await press("ArrowDown");
    expect(video.volume).toBeCloseTo(0.5);
    await act(async () => {
      video.volume = 0.02;
    });
    await press("ArrowDown");
    expect(video.volume).toBe(0);
    await act(async () => {
      video.volume = 0.98;
    });
    await press("ArrowUp");
    expect(video.volume).toBe(1);
  });

  it("leaves live playback timing and speed alone", async () => {
    props.isLive = true;
    await render();
    for (const key of [
      ",",
      ".",
      "<",
      ">",
      "j",
      "l",
      "ArrowLeft",
      "ArrowRight",
      "0",
      "Home",
      "End",
    ]) {
      expect((await press(key)).defaultPrevented).toBe(false);
    }
    expect(video.currentTime).toBe(50);
    expect(video.playbackRate).toBe(1);
  });

  it.each([NaN, Infinity, 0])(
    "does not seek before a finite duration is known (%s)",
    async (duration) => {
      Object.defineProperty(video, "duration", { value: duration });
      await press("l");
      await press("9");
      await press(".");
      expect(video.currentTime).toBe(50);
    },
  );

  it.each(["input", "textarea", "select", "button", "a"])(
    "preserves shortcuts on focused %s elements",
    async (tag) => {
      const target = document.createElement(tag);
      container.append(target);
      expect((await press(" ", target)).defaultPrevented).toBe(false);
      expect(video.paused).toBe(true);
    },
  );

  it.each(["slider", "menu", "dialog", "textbox"])(
    "leaves %s keyboard handling alone",
    async (role) => {
      const target = document.createElement("div");
      target.setAttribute("role", role);
      const child = document.createElement("span");
      target.append(child);
      container.append(target);
      expect((await press("ArrowRight", child)).defaultPrevented).toBe(false);
      expect(video.currentTime).toBe(50);
    },
  );

  it("leaves rich text editing alone, including nested targets", async () => {
    const editor = document.createElement("div");
    editor.contentEditable = "true";
    editor.setAttribute("contenteditable", "true");
    const child = document.createElement("span");
    editor.append(child);
    container.append(editor);
    await press("l", child);
    expect(video.currentTime).toBe(50);
  });

  it.each([
    { ctrlKey: true },
    { metaKey: true },
    { altKey: true },
    { isComposing: true },
  ])("preserves modified and composing key events (%o)", async (options) => {
    expect((await press("k", document.body, options)).defaultPrevented).toBe(
      false,
    );
    expect(video.paused).toBe(true);
  });

  it("does not double-handle keys consumed by other controls", async () => {
    const event = new KeyboardEvent("keydown", {
      key: " ",
      bubbles: true,
      cancelable: true,
    });
    event.preventDefault();
    await act(async () => document.body.dispatchEvent(event));
    expect(video.paused).toBe(true);
  });

  async function click(label: string) {
    const button = Array.from(
      document.querySelectorAll<HTMLElement>("button, [role^='menuitem']"),
    ).find(
      (element) =>
        element.getAttribute("aria-label") === label ||
        element.textContent?.trim() === label,
    );
    expect(button, `missing control: ${label}`).toBeDefined();
    await act(async () => button!.click());
  }

  it("selects speed in settings and mirrors rate changes from shortcuts", async () => {
    await press(">");
    await click("Settings");
    await click("Playback speed");
    const selected = Array.from(
      document.querySelectorAll('[role="menuitemradio"][aria-checked="true"]'),
    );
    expect(selected.some((element) => element.textContent === "1.25×")).toBe(
      true,
    );
    await click("1.5×");
    expect(video.playbackRate).toBe(1.5);
  });

  it("offers paused frame controls without shortcut hints in settings", async () => {
    await click("Settings");
    expect(document.body.textContent).not.toContain("Keyboard shortcuts");
    await click("Frame by frame");
    expect(document.body.textContent).not.toContain("Pause first, then use");
    await click("Next frame");
    expect(video.currentTime).toBeCloseTo(50 + 1 / 60);
  });

  it("hides recording controls in live settings", async () => {
    props.isLive = true;
    await render();
    await click("Settings");
    expect(document.body.textContent).not.toContain("Playback speed");
    expect(document.body.textContent).not.toContain("Frame by frame");
  });

  it("disables the frame menu while playing", async () => {
    await act(async () => video.play());
    await click("Settings");
    await click("Frame by frame");
    const item = Array.from(
      document.querySelectorAll('[role="menuitem"]'),
    ).find((element) => element.textContent === "Next frame");
    expect(item?.getAttribute("aria-disabled")).toBe("true");
  });

  describe("action feedback", () => {
    beforeEach(() => vi.useFakeTimers());

    function feedback() {
      return document.querySelector('[role="status"]');
    }

    it("immediately replaces the last action with the latest result", async () => {
      await press("l");
      expect(feedback()?.textContent).toBe("Seek to 1:00");
      expect(feedback()?.querySelector(".lucide-fast-forward")).not.toBeNull();
      await press(">");
      expect(feedback()?.textContent).toBe("Playback speed: 1.25×");
      expect(feedback()?.querySelector(".lucide-rabbit")).not.toBeNull();
      await press("j");
      expect(feedback()?.textContent).toBe("Seek to 0:50");
      expect(feedback()?.querySelector(".lucide-rewind")).not.toBeNull();
      expect(feedback()?.querySelectorAll("svg")).toHaveLength(1);
      expect(document.querySelectorAll('[role="status"]')).toHaveLength(1);
    });

    it.each([
      { start: 2, key: "<", rate: 1.75, icon: "rabbit" },
      { start: 0.25, key: ">", rate: 0.5, icon: "snail" },
      { start: 1.25, key: "<", rate: 1, icon: "panda" },
      { start: 0.75, key: ">", rate: 1, icon: "panda" },
    ])(
      "shows $icon at $rate× after $key",
      async ({ start, key, rate, icon }) => {
        await act(async () => {
          video.playbackRate = start;
        });
        await press(key);
        expect(feedback()?.textContent).toBe(`Playback speed: ${rate}×`);
        expect(feedback()?.querySelector(`.lucide-${icon}`)).not.toBeNull();
      },
    );

    it("updates frame position without waiting for a media timeupdate", async () => {
      await press(".");
      expect(feedback()?.textContent).toBe("Next frame at 0:50.017");
      expect(feedback()?.querySelector(".lucide-step-forward")).not.toBeNull();
      await press(".");
      expect(feedback()?.textContent).toBe("Next frame at 0:50.033");
      await press(",");
      expect(feedback()?.textContent).toBe("Previous frame at 0:50.017");
      expect(feedback()?.querySelector(".lucide-step-back")).not.toBeNull();
    });

    it("hides the centered play icon during feedback without disabling its control", async () => {
      props.bigPlay = true;
      await render();
      const playButton = container.querySelector(
        'button[aria-label="Play"][aria-hidden]',
      );
      expect(playButton?.querySelector("svg")).not.toBeNull();
      await press(".");
      expect(playButton?.querySelector("svg")).toBeNull();
      expect(playButton?.getAttribute("aria-hidden")).toBe("false");
      await act(async () => vi.advanceTimersByTime(1_500));
      expect(playButton?.querySelector("svg")).not.toBeNull();
      await press(".");
      await act(async () => (playButton as HTMLButtonElement).click());
      expect(video.paused).toBe(false);
    });

    it("confirms playback, mute, and resulting volume", async () => {
      await press("k");
      expect(feedback()?.textContent).toBe("Play");
      await press("k");
      expect(feedback()?.textContent).toBe("Pause");
      await press("m");
      expect(feedback()?.textContent).toBe("Muted");
      await press("m");
      expect(feedback()?.textContent).toBe("Unmuted");
      await press("ArrowDown");
      expect(feedback()?.textContent).toBe("Volume: 95%");
    });

    it("resets dismissal even when the repeated message is identical", async () => {
      await press("ArrowUp");
      await act(async () => vi.advanceTimersByTime(1_000));
      await press("ArrowUp");
      expect(feedback()?.textContent).toBe("Volume: 100%");
      await act(async () => vi.advanceTimersByTime(500));
      expect(feedback()?.textContent).toBe("Volume: 100%");
      await act(async () => vi.advanceTimersByTime(1_000));
      expect(feedback()).toBeNull();
    });

    it("does not let an older dismissal hide a newer action", async () => {
      await press(">");
      await act(async () => vi.advanceTimersByTime(1_000));
      await press("l");
      await act(async () => vi.advanceTimersByTime(500));
      expect(feedback()?.textContent).toBe("Seek to 1:00");
    });

    it("does not show feedback for unavailable live actions or typing", async () => {
      props.isLive = true;
      await render();
      await press(".");
      await press(">");
      await press("l");
      const input = document.createElement("input");
      container.append(input);
      await press("k", input);
      expect(feedback()).toBeNull();
      await press("ArrowDown");
      expect(feedback()?.textContent).toBe("Volume: 95%");
    });

    it("cancels its dismissal timer on unmount", async () => {
      await press(">");
      await act(async () => root.unmount());
      expect(vi.getTimerCount()).toBe(0);
    });
  });
});
