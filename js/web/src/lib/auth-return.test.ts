import { afterEach, describe, expect, it } from "vitest";
import {
  consumeAuthReturnPath,
  sanitizeAuthReturnPath,
  saveAuthReturnPath,
} from "./auth-return";

afterEach(() => sessionStorage.clear());

describe("sanitizeAuthReturnPath", () => {
  it("keeps local app paths including search and hash", () => {
    expect(sanitizeAuthReturnPath("/chicago.at/video/abc?t=4#chat")).toBe(
      "/chicago.at/video/abc?t=4#chat",
    );
  });

  it("rejects absolute, protocol-relative, and login return paths", () => {
    expect(sanitizeAuthReturnPath("https://evil.example/phish")).toBeNull();
    expect(sanitizeAuthReturnPath("//evil.example/phish")).toBeNull();
    expect(sanitizeAuthReturnPath("/login?loop=1")).toBeNull();
  });
});

describe("auth return storage", () => {
  it("consumes the path once", () => {
    saveAuthReturnPath("/natalie.sh");

    expect(consumeAuthReturnPath()).toBe("/natalie.sh");
    expect(consumeAuthReturnPath()).toBeNull();
  });

  it("does not block authentication when browser storage is unavailable", () => {
    const unavailableStorage = {
      getItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
      setItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
      removeItem: () => {
        throw new DOMException("blocked", "SecurityError");
      },
    };

    expect(() =>
      saveAuthReturnPath("/natalie.sh", unavailableStorage),
    ).not.toThrow();
    expect(consumeAuthReturnPath(unavailableStorage)).toBeNull();
  });
});
