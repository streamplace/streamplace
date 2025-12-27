import type { LinkParams } from "components/aqlink";

const SETTINGS_SCREENS = [
  "MainSettings",
  "AboutCategory",
  "AccountCategory",
  "StreamingCategory",
  "WebhooksSettings",
  "PrivacyCategory",
  "DanmuCategory",
  "AdvancedCategory",
  "LanguagesCategory",
  "KeyManagement",
];

// Screens that are in the HomeTab stack
const HOME_TAB_SCREENS = [
  "HomeMain",
  "About",
  "Download",
  "LiveDashboard",
  "Login",
  "Multi",
  "Support",
];

/**
 * Converts navigation params to nested tab structure.
 * Most screens are inside HomeTab or SettingsTab, only full-screen experiences like
 * Stream and MobileGoLive are at root stack level.
 */
export function convertNavigationParams(to: LinkParams): LinkParams {
  // Handle settings screens - nest them in SettingsTab
  if (SETTINGS_SCREENS.includes(to.screen)) {
    return {
      screen: "MainTabs",
      params: {
        screen: "SettingsTab",
        params: {
          screen: to.screen,
          params: to.params,
        },
      },
    };
  }

  // Handle screens that are in HomeTab (includes both current and legacy names)
  // Legacy: StreamList → HomeMain
  const homeScreen = to.screen === "StreamList" ? "HomeMain" : to.screen;

  if (HOME_TAB_SCREENS.includes(homeScreen)) {
    return {
      screen: "MainTabs",
      params: {
        screen: "HomeTab",
        params: {
          screen: homeScreen,
          params: to.params,
        },
      },
    };
  }

  // GoLiveTab
  if (to.screen === "LaunchGoLive" || to.screen === "GoLiveTab") {
    return {
      screen: "MainTabs",
      params: {
        screen: "GoLiveTab",
      },
    };
  }

  // All other screens (Stream, MobileGoLive, embeds, etc.) are at root stack level
  return to;
}
