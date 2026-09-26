import { describe, expect, it } from "vitest";
import {
  CHAT_MESSAGE_FADE_DURATION_MS,
  CHAT_MESSAGE_FADE_START_MS,
  CHAT_MESSAGE_GONE_MS,
  chatMessageOpacity,
  isChatMessageGone,
  liveChatView,
} from "./chat-expiry";

// Fixed clock so age arithmetic is exact rather than wall-clock dependent.
const NOW = Date.UTC(2024, 0, 1, 12, 0, 0);
const iso = (offsetMs: number) => new Date(NOW + offsetMs).toISOString();

// A message that claims to be from `createdOffsetMs` and was taken by the node
// at `indexedOffsetMs` (default: the same moment).
const message = (createdOffsetMs: number, indexedOffsetMs?: number) => ({
  record: { createdAt: iso(createdOffsetMs) },
  indexedAt: indexedOffsetMs === undefined ? undefined : iso(indexedOffsetMs),
});

describe("chatMessageOpacity", () => {
  it("leaves a fresh message at full strength", () => {
    expect(chatMessageOpacity(message(0, 0), NOW)).toBe(1);
    expect(
      chatMessageOpacity(message(-CHAT_MESSAGE_FADE_START_MS + 1, 0), NOW),
    ).toBe(1);
  });

  it("starts fading exactly at the fade start", () => {
    expect(
      chatMessageOpacity(message(-CHAT_MESSAGE_FADE_START_MS, 0), NOW),
    ).toBe(1);
  });

  it("is half faded at the midpoint of the fade window", () => {
    const half = CHAT_MESSAGE_FADE_START_MS + CHAT_MESSAGE_FADE_DURATION_MS / 2;
    expect(chatMessageOpacity(message(-half, 0), NOW)).toBeCloseTo(0.5);
  });

  it("is gone at the end of the fade window and stays gone", () => {
    expect(chatMessageOpacity(message(-CHAT_MESSAGE_GONE_MS, 0), NOW)).toBe(0);
    expect(
      chatMessageOpacity(message(-(CHAT_MESSAGE_GONE_MS + 86_400_000), 0), NOW),
    ).toBe(0);
  });

  it("ages a backfilled message from when it was said, not when it arrived", () => {
    const said = -(3 * 60 * 60 * 1000);
    expect(chatMessageOpacity(message(said, 0), NOW)).toBe(0);
  });

  it("ages a message post-dated into the future from when the node took it", () => {
    const future = 10 * 60 * 60 * 1000;
    expect(chatMessageOpacity(message(future, 0), NOW)).toBe(1);
    expect(
      chatMessageOpacity(message(future, 0), NOW + 90 * 60_000),
    ).toBeCloseTo(0.5);
    expect(
      chatMessageOpacity(message(future, 0), NOW + CHAT_MESSAGE_GONE_MS),
    ).toBe(0);
  });

  it("treats an unreadable timestamp as fresh rather than hiding it", () => {
    expect(
      chatMessageOpacity({ record: { createdAt: "not a date" } }, NOW),
    ).toBe(1);
    expect(chatMessageOpacity({}, NOW)).toBe(1);
  });
});

describe("isChatMessageGone", () => {
  it("keeps messages until the fade finishes", () => {
    expect(isChatMessageGone(message(0, 0), NOW)).toBe(false);
    expect(
      isChatMessageGone(message(-(CHAT_MESSAGE_GONE_MS - 1), 0), NOW),
    ).toBe(false);
  });

  it("drops a message once the fade finishes", () => {
    expect(isChatMessageGone(message(-CHAT_MESSAGE_GONE_MS, 0), NOW)).toBe(
      true,
    );
    expect(
      isChatMessageGone(message(-(CHAT_MESSAGE_GONE_MS + 1), 0), NOW),
    ).toBe(true);
  });

  it("cannot be pinned to the live view by post-dating the record", () => {
    const future = 10 * 60 * 60 * 1000;
    expect(isChatMessageGone(message(future, 0), NOW)).toBe(false);
    expect(
      isChatMessageGone(message(future, 0), NOW + CHAT_MESSAGE_GONE_MS),
    ).toBe(true);
  });

  it("keeps a message with an unreadable timestamp", () => {
    expect(
      isChatMessageGone({ record: { createdAt: "not a date" } }, NOW),
    ).toBe(false);
    expect(isChatMessageGone({}, NOW)).toBe(false);
  });
});

describe("liveChatView", () => {
  const fresh = message(-60_000, 0);
  const old = message(-(3 * 60 * 60 * 1000), 0);

  it("shows only live messages at the live edge", () => {
    const view = liveChatView([fresh, old], NOW, false);
    expect(view.history).toBe(false);
    expect(view.messages).toEqual([fresh]);
  });

  it("shows the whole retained list when the viewer is reading history", () => {
    const view = liveChatView([fresh, old], NOW, true);
    expect(view.history).toBe(true);
    expect(view.messages).toEqual([fresh, old]);
  });

  it("falls back to history when nothing live is left to scroll", () => {
    const view = liveChatView([old], NOW, false);
    expect(view.history).toBe(true);
    expect(view.messages).toEqual([old]);
  });

  it("is history rather than live for an empty list", () => {
    const view = liveChatView<ReturnType<typeof message>>([], NOW, false);
    expect(view.history).toBe(true);
    expect(view.messages).toEqual([]);
  });
});
