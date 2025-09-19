import { beforeAll, beforeEach, describe, it } from "@jest/globals";
import { by, element, expect } from "detox";

describe("Extremely basic e2e test", () => {
  beforeAll(async () => {
    await device.launchApp();
  });

  beforeEach(async () => {
    await device.reloadReactNative();
  });

  it("should have welcome screen", async () => {
    await expect(element(by.id("sidebar-button"))).toBeVisible();
    await element(by.id("sidebar-button")).tap();
    await expect(element(by.id("settings-button"))).toBeVisible();
    await element(by.id("settings-button")).tap();
  });
});
