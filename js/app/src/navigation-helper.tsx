import type { LinkParams } from "components/aqlink";
import { Platform } from "react-native";

const SETTINGS_SCREENS = [
  "MainSettings",
  "AboutCategory",
  "AccountCategory",
  "BackupSettings",
  "StreamingCategory",
  "WebhooksSettings",
  "RecommendationsSettings",
  "PrivacyCategory",
  "DanmuCategory",
  "AdvancedCategory",
  "LanguagesCategory",
  "MultistreamCategory",
  "KeyManagement",
  "BadgeSelection",
  "BadgeIssuer",
  "BrandingAdmin",
];

// Screens that are in the HomeTab stack
const HOME_TAB_SCREENS = [
  "HomeMain",
  "Download",
  "LiveDashboard",
  "Login",
  "Multi",
  "Support",
  "Upload",
  "UploadVideo",
  "UploadDrafts",
  "UploadLivestreams",
  "UploadVideos",
];

// Screens that live in the VideosTab stack
const VIDEOS_TAB_SCREENS = ["VideoList", "UserVideoList"];

// Screens at root stack level (need special navigation from nested navigators)
export const ROOT_SCREENS = [
  "Creator",
  "Stream",
  "MobileGoLive",
  "AppReturn",
  "PopoutChat",
  "Embed",
  "InfoWidgetEmbed",
  "DanmuOBS",
  "Video",
  "Vod",
  "VodEmbed",
];

/**
 * Converts navigation params to nested tab structure.
 * Most screens are inside HomeTab or SettingsTab, only full-screen experiences like
 * Stream and MobileGoLive are at root stack level.
 */
export function convertNavigationParams(to: LinkParams): LinkParams {
  if (
    Platform.OS === "web" &&
    (to.screen === "LiveDashboard" || to.screen === "CreatorControlPanel")
  ) {
    return {
      screen: "Creator",
      params: { screen: "CreatorControlPanel", params: to.params },
    };
  }

  if (to.screen === "CreatorControlPanel") {
    to = { screen: "LiveDashboard", params: to.params };
  }

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

  // Handle screens that are in VideosTab
  if (VIDEOS_TAB_SCREENS.includes(to.screen)) {
    return {
      screen: "MainTabs",
      params: {
        screen: "VideosTab",
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
