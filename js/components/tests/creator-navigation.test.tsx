import { getPathFromState } from "@react-navigation/core";
import { beforeEach, expect, it, vi } from "vitest";
import {
  getStreamplaceStateFromPath,
  streamplaceLinkingOptions,
} from "../../app/src/linking-config";
import { convertNavigationParams } from "../../app/src/navigation-helper";

const platform = vi.hoisted(() => ({ OS: "web" }));
// The native entry also exports platform views. Keep the real routing engine
// without loading those presentation bindings in jsdom.
vi.mock("@react-navigation/native", () => import("@react-navigation/core"));
vi.mock("react-native", () => ({
  Platform: platform,
  I18nManager: { isRTL: false },
}));
vi.mock("expo-linking", () => ({ createURL: () => "streamplace://" }));

beforeEach(() => {
  platform.OS = "web";
});

function activeNames(state: any): string[] {
  const route = state.routes[state.index ?? state.routes.length - 1];
  return [route.name, ...(route.state ? activeNames(route.state) : [])];
}

it.each(["/dashboard", "/live", "/dashboard?from=obs"])(
  "opens the browser creator host from %s",
  (path) => {
    const state = getStreamplaceStateFromPath(path);
    expect(activeNames(state)).toEqual(["Creator", "CreatorControlPanel"]);
    expect(getPathFromState(state, streamplaceLinkingOptions.config)).toMatch(
      /^\/dashboard(?:\?|$)/,
    );
  },
);

it("routes existing dashboard links through the browser creator host", () => {
  expect(convertNavigationParams({ screen: "LiveDashboard" })).toEqual({
    screen: "Creator",
    params: { screen: "CreatorControlPanel", params: undefined },
  });
});

it("keeps native live navigation inside the home tab", () => {
  platform.OS = "ios";
  expect(convertNavigationParams({ screen: "LiveDashboard" })).toEqual({
    screen: "MainTabs",
    params: {
      screen: "HomeTab",
      params: { screen: "LiveDashboard", params: undefined },
    },
  });
});

it.each(["ios", "android"])(
  "resolves dashboard URLs to the native live screen on %s",
  async (os) => {
    platform.OS = os;
    vi.resetModules();
    const nativeLinking = await import("../../app/src/linking-config");
    for (const path of ["/live", "/dashboard"]) {
      expect(
        activeNames(nativeLinking.getStreamplaceStateFromPath(path)),
      ).toEqual(["MainTabs", "HomeTab", "LiveDashboard"]);
    }
  },
);

it.each([
  ["/settings/privacy", ["MainTabs", "SettingsTab", "PrivacyCategory"]],
  ["/widgets/livestream", ["PopoutLivestream"]],
  ["/chat-popout/alice.example", ["PopoutChat"]],
  ["/alice.example/video/record", ["Video"]],
  ["/at://did:plc:alice/place.stream.video/record", ["Video"]],
])("retains the existing destination for %s", (path, names) => {
  expect(activeNames(getStreamplaceStateFromPath(path as string))).toEqual(
    names,
  );
});
