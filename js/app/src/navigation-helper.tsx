import type { LinkParams } from "components/aqlink";
import { Platform } from "react-native";

// Settings screens that are nested in the SettingsStack on iOS
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

/**
 * Converts navigation params to platform-specific format.
 *
 * Web uses flat navigation: { screen: "AccountCategory" }
 * iOS uses nested navigation: { screen: "MainTabs", params: { screen: "SettingsTab", params: { screen: "AccountCategory" } } }
 */
export function convertNavigationParams(to: LinkParams): LinkParams {
  // On web, use flat navigation
  if (Platform.OS === "web") {
    return to;
  }

  // On iOS, handle nested navigation for settings screens
  if (Platform.OS === "ios") {
    // If navigating to a settings screen, nest it properly
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

    // StreamList is in HomeTab
    if (to.screen === "StreamList") {
      return {
        screen: "MainTabs",
        params: {
          screen: "HomeTab",
        },
      };
    }

    // LaunchGoLive is in GoLiveTab
    if (to.screen === "LaunchGoLive") {
      return {
        screen: "MainTabs",
        params: {
          screen: "GoLiveTab",
        },
      };
    }

    // All other screens are at root level
    return to;
  }

  // Android and other platforms use flat navigation like web
  return to;
}
