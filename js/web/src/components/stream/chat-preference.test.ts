import { describe, expect, it } from "vitest";
import {
  chatOpenAfterLivenessChange,
  chatPreferenceKey,
  shouldOpenChat,
} from "./chat-preference";

describe("chat preference", () => {
  it("only persists the desktop preference", () => {
    expect(chatPreferenceKey(true)).toBe("streamplace:chat-open");
    expect(chatPreferenceKey(false)).toBeNull();
  });

  it("opens live chat by default", () => {
    expect(shouldOpenChat(false, null)).toBe(true);
  });

  it("respects an explicit closed preference while live", () => {
    expect(shouldOpenChat(false, "false")).toBe(false);
  });

  it("keeps chat closed while offline", () => {
    expect(shouldOpenChat(true, null)).toBe(false);
    expect(shouldOpenChat(true, "true")).toBe(false);
  });

  it("restores the intended chat state after a false-offline startup", () => {
    expect(
      chatOpenAfterLivenessChange({
        isOffline: false,
        wasOffline: true,
        userChangedChat: false,
        preferredChatOpen: true,
        currentChatOpen: false,
      }),
    ).toBe(true);

    expect(
      chatOpenAfterLivenessChange({
        isOffline: false,
        wasOffline: true,
        userChangedChat: false,
        preferredChatOpen: false,
        currentChatOpen: false,
      }),
    ).toBe(false);
  });

  it("does not override a viewer's choice during the visit", () => {
    expect(
      chatOpenAfterLivenessChange({
        isOffline: false,
        wasOffline: true,
        userChangedChat: true,
        preferredChatOpen: true,
        currentChatOpen: false,
      }),
    ).toBe(false);
  });

  it("closes chat when a live stream goes offline", () => {
    expect(
      chatOpenAfterLivenessChange({
        isOffline: true,
        wasOffline: false,
        userChangedChat: false,
        preferredChatOpen: true,
        currentChatOpen: true,
      }),
    ).toBe(false);
  });
});
