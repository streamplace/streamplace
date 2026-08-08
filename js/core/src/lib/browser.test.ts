import { describe, expect, it } from "vitest";
import { getBrowserName } from "./browser";

describe("getBrowserName", () => {
  it("detects Firefox", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (X11; Linux i686; rv:104.0) Gecko/20100101 Firefox/104.0",
      ),
    ).toBe("Firefox");
  });

  it("detects Samsung Internet", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (Linux; Android 9; SAMSUNG SM-G955F) SamsungBrowser/9.4 Chrome/67.0.3396.87 Mobile Safari/537.36",
      ),
    ).toBe("Samsung Internet");
  });

  it("detects Opera via OPR string", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 12_5_1) AppleWebKit/537.36 Chrome/104.0.0.0 Safari/537.36 OPR/90.0.4480.54",
      ),
    ).toBe("Opera");
  });

  it("detects Opera via Opera string", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/104.0.0.0 Safari/537.36 OPR/90.0",
      ),
    ).toBe("Opera");
  });

  it("detects legacy Edge", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/58.0.3029.110 Safari/537.36 Edge/16.16299",
      ),
    ).toBe("Edge (Legacy)");
  });

  it("detects Chromium Edge", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/104.0.0.0 Safari/537.36 Edg/104.0.1293.70",
      ),
    ).toBe("Edge (Chromium)");
  });

  it("detects Chrome", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/104.0.0.0 Safari/537.36",
      ),
    ).toBe("Chrome");
  });

  it("detects Safari", () => {
    expect(
      getBrowserName(
        "Mozilla/5.0 (iPhone; CPU iPhone OS 15_6_1) AppleWebKit/605.1.15 Safari/604.1",
      ),
    ).toBe("Safari");
  });

  it("returns unknown for empty string", () => {
    expect(getBrowserName("")).toBe("unknown");
  });

  it("returns unknown for unrecognised UA", () => {
    expect(getBrowserName("curl/7.84.0")).toBe("unknown");
  });
});
