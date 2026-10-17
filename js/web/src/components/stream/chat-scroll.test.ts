import { describe, expect, it } from "vitest";
import { initializeChatScroll } from "./chat-scroll";

function scrollBox({
  scrollHeight,
  clientHeight,
  scrollTop = 0,
}: {
  scrollHeight: number;
  clientHeight: number;
  scrollTop?: number;
}) {
  return { scrollHeight, clientHeight, scrollTop };
}

describe("initializeChatScroll", () => {
  it("starts normal chat at the newest message", () => {
    const element = scrollBox({ scrollHeight: 2400, clientHeight: 500 });

    expect(initializeChatScroll(element, false)).toBe(true);
    expect(element.scrollTop).toBe(2400);
  });

  it("starts reversed chat at the top anchor", () => {
    const element = scrollBox({
      scrollHeight: 2400,
      clientHeight: 500,
      scrollTop: 700,
    });

    expect(initializeChatScroll(element, true)).toBe(true);
    expect(element.scrollTop).toBe(0);
  });

  it("waits when the panel is still hidden", () => {
    const element = scrollBox({ scrollHeight: 2400, clientHeight: 0 });

    expect(initializeChatScroll(element, false)).toBe(false);
    expect(element.scrollTop).toBe(0);
  });
});
