import type { LinkParams } from "components/aqlink";

const SETTINGS_SCREENS = [
  "MainSettings",
  "AboutCategory",
  "AccountCategory",
  "BackupSettings",
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

// Screens at root stack level (need special navigation from nested navigators)
export const ROOT_SCREENS = [
  "Stream",
  "MobileGoLive",
  "AppReturn",
  "PopoutChat",
  "Embed",
  "InfoWidgetEmbed",
  "DanmuOBS",
  "AVSync",
  "LegacyStream",
  "VodPlayerDemo",
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

  // Handle screens that are in HomeTab
  if (HOME_TAB_SCREENS.includes(to.screen)) {
    return {
      screen: "MainTabs",
      params: {
        screen: "HomeTab",
        params: {
          screen: to.screen,
          params: to.params,
        },
      },
    };
  }

  // GoLiveTab
  if (to.screen === "GoLiveTab") {
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
