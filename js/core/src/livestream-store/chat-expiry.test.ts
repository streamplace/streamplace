import { describe, expect, it } from "vitest";
import {
  CHAT_MESSAGE_FADE_DURATION_MS,
  CHAT_MESSAGE_FADE_START_MS,
  CHAT_MESSAGE_GONE_MS,
  chatMessageOpacity,
  isChatMessageGone,
} from "./chat-expiry";

// Fixed clock so age arithmetic is exact rather than wall-clock dependent.
const NOW = Date.UTC(2024, 0, 1, 12, 0, 0);
const createdAtMsAgo = (ms: number) => new Date(NOW - ms).toISOString();

describe("chatMessageOpacity", () => {
  it("leaves a fresh message at full strength", () => {
    expect(chatMessageOpacity(createdAtMsAgo(0), NOW)).toBe(1);
    expect(
      chatMessageOpacity(createdAtMsAgo(CHAT_MESSAGE_FADE_START_MS - 1), NOW),
    ).toBe(1);
  });

  it("starts fading exactly at the fade start", () => {
    expect(
      chatMessageOpacity(createdAtMsAgo(CHAT_MESSAGE_FADE_START_MS), NOW),
    ).toBe(1);
  });

  it("is half faded at the midpoint of the fade window", () => {
    const half = CHAT_MESSAGE_FADE_START_MS + CHAT_MESSAGE_FADE_DURATION_MS / 2;
    expect(chatMessageOpacity(createdAtMsAgo(half), NOW)).toBeCloseTo(0.5);
  });

  it("is gone at the end of the fade window and stays gone", () => {
    expect(chatMessageOpacity(createdAtMsAgo(CHAT_MESSAGE_GONE_MS), NOW)).toBe(
      0,
    );
    expect(
      chatMessageOpacity(
        createdAtMsAgo(CHAT_MESSAGE_GONE_MS + 86_400_000),
        NOW,
      ),
    ).toBe(0);
  });

  it("does not fade a message timestamped in the future", () => {
    expect(chatMessageOpacity(new Date(NOW + 60_000).toISOString(), NOW)).toBe(
      1,
    );
  });

  it("treats an unreadable timestamp as fresh rather than hiding it", () => {
    expect(chatMessageOpacity("not a date", NOW)).toBe(1);
  });
});

describe("isChatMessageGone", () => {
  it("keeps messages until the fade finishes", () => {
    expect(isChatMessageGone(createdAtMsAgo(0), NOW)).toBe(false);
    expect(
      isChatMessageGone(createdAtMsAgo(CHAT_MESSAGE_GONE_MS - 1), NOW),
    ).toBe(false);
  });

  it("drops a message once the fade finishes", () => {
    expect(isChatMessageGone(createdAtMsAgo(CHAT_MESSAGE_GONE_MS), NOW)).toBe(
      true,
    );
    expect(
      isChatMessageGone(createdAtMsAgo(CHAT_MESSAGE_GONE_MS + 1), NOW),
    ).toBe(true);
  });

  it("keeps a message with an unreadable timestamp", () => {
    expect(isChatMessageGone("not a date", NOW)).toBe(false);
  });
});
