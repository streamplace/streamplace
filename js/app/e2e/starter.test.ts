import { beforeAll, beforeEach, describe, it } from "@jest/globals";
import { by, element, expect } from "detox";

describe("Example", () => {
  beforeAll(async () => {
    await device.launchApp();
  });

  beforeEach(async () => {
    // await device.reloadReactNative();
  });

  it("should have welcome screen", async () => {
    await expect(element(by.id("sidebar-button"))).toBeVisible();
    await element(by.id("sidebar-button")).tap();
    await expect(element(by.id("settings-button"))).toBeVisible();
    await element(by.id("settings-button")).tap();
  });

  // it("should show hello screen after tap", async () => {
  //   await element(by.id("hello_button")).tap();
  //   await expect(element(by.text("Hello!!!"))).toBeVisible();
  // });

  // it("should show world screen after tap", async () => {
  //   await element(by.id("world_button")).tap();
  //   await expect(element(by.text("World!!!"))).toBeVisible();
  // });
});
