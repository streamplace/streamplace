import { describe, expect, it } from "vitest";
import {
  isWebBetaEnabled,
  setWebBetaEnabled,
  WEB_BETA_COOKIE,
} from "./web-beta";

describe("web-beta cookie", () => {
  it("exposes the cookie name", () => {
    expect(WEB_BETA_COOKIE).toBe("sp_web_beta");
  });

  it("returns false when cookie is not set", () => {
    document.cookie = "sp_web_beta=; max-age=0";
    expect(isWebBetaEnabled()).toBe(false);
  });

  it("returns true after enabling", () => {
    setWebBetaEnabled(true);
    expect(isWebBetaEnabled()).toBe(true);
  });

  it("returns false after disabling", () => {
    setWebBetaEnabled(true);
    setWebBetaEnabled(false);
    expect(isWebBetaEnabled()).toBe(false);
  });

  it("returns false for cookie value other than 1", () => {
    document.cookie = "sp_web_beta=0; path=/";
    expect(isWebBetaEnabled()).toBe(false);
  });

  it("returns false for garbage cookie value", () => {
    document.cookie = "sp_web_beta=garbage; path=/";
    expect(isWebBetaEnabled()).toBe(false);
  });
});
