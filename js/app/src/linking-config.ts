/**
 * Shared linking configuration for react-navigation
 * Used both for URL parsing (inbound) and URL generation (outbound)
 */

import { LinkingOptions, getStateFromPath } from "@react-navigation/native";
import * as ExpoLinking from "expo-linking";

export const SCREEN_PATHS = {
  // HomeTab screens
  HomeMain: "",
  About: "about",
  Download: "download",
  LiveDashboard: "live",
  Login: "login",
  Multi: "multi/:config",
  Support: "support",
  // Settings screens
  MainSettings: "settings",
  AboutCategory: "settings/about",
  AccountCategory: "settings/account",
  BackupSettings: "/settings/streaming/backup",
  StreamingCategory: "settings/streaming",
  WebhooksSettings: "settings/streaming/webhooks",
  RecommendationsSettings: "settings/streaming/recommendations",
  PrivacyCategory: "settings/privacy",
  DanmuCategory: "settings/danmu",
  AdvancedCategory: "settings/advanced",
  DeveloperSettings: "settings/developer",
  MultistreamCategory: "settings/streaming/multistream",
  KeyManagement: "settings/streaming/key-management",
  LanguagesCategory: "settings/languages",
  BrandingAdmin: "settings/branding",
  // Tabs
  GoLiveTab: "go-live",
  // Root stack screens
  Stream: ":user",
  MobileGoLive: "mobile-golive",
  AVSync: "sync-test",
  AppReturn: "app-return/:scheme",
  PopoutChat: "chat-popout/:user",
  Embed: "embed/:user",
  InfoWidgetEmbed: "info-widget",
  LegacyStream: "legacy/:user",
  DanmuOBS: "widgets/:user/danmu",
} as const;

/**
 * Convert screen path to absolute URL path
 * Adds leading slash if not present
 */
export function getAbsolutePath(screenName: keyof typeof SCREEN_PATHS): string {
  const path = SCREEN_PATHS[screenName];
  return path.startsWith("/") ? path : `/${path}`;
}

/**
 * Interpolate params into a path template
 * Example: interpolateParams("/:user", { user: "alice" }) => "/alice"
 */
export function interpolateParams(
  path: string,
  params?: Record<string, any>,
): string {
  if (!params || typeof params !== "object") {
    return path;
  }

  let result = path;
  for (const [key, value] of Object.entries(params)) {
    result = result.replace(`:${key}`, String(value));
  }
  return result;
}

/**
 * Check if a screen name is valid
 */
export function isValidScreenName(
  name: string,
): name is keyof typeof SCREEN_PATHS {
  return name in SCREEN_PATHS;
}

export const streamplaceLinkingOptions: LinkingOptions<ReactNavigation.RootParamList> =
  {
    prefixes: [ExpoLinking.createURL("")],
    config: {
      screens: {
        // Main tabs (used on all platforms, tab bar hidden on web)
        MainTabs: {
          screens: {
            HomeTab: {
              screens: {
                HomeMain: SCREEN_PATHS.HomeMain,
                About: SCREEN_PATHS.About,
                Download: SCREEN_PATHS.Download,
                LiveDashboard: SCREEN_PATHS.LiveDashboard,
                Login: SCREEN_PATHS.Login,
                Multi: SCREEN_PATHS.Multi,
                Support: SCREEN_PATHS.Support,
              },
            },
            GoLiveTab: SCREEN_PATHS.GoLiveTab,
            SettingsTab: {
              screens: {
                MainSettings: SCREEN_PATHS.MainSettings,
                AboutCategory: SCREEN_PATHS.AboutCategory,
                AccountCategory: SCREEN_PATHS.AccountCategory,
                BackupSettings: SCREEN_PATHS.BackupSettings,
                StreamingCategory: SCREEN_PATHS.StreamingCategory,
                WebhooksSettings: SCREEN_PATHS.WebhooksSettings,
                RecommendationsSettings: SCREEN_PATHS.RecommendationsSettings,
                PrivacyCategory: SCREEN_PATHS.PrivacyCategory,
                DanmuCategory: SCREEN_PATHS.DanmuCategory,
                AdvancedCategory: SCREEN_PATHS.AdvancedCategory,
                DeveloperSettings: SCREEN_PATHS.DeveloperSettings,
                MultistreamCategory: SCREEN_PATHS.MultistreamCategory,
                KeyManagement: SCREEN_PATHS.KeyManagement,
                LanguagesCategory: SCREEN_PATHS.LanguagesCategory,
                BrandingAdmin: SCREEN_PATHS.BrandingAdmin,
              },
            },
          },
        },
        // Root stack screens (outside tabs - full-screen experiences)
        Stream: {
          path: SCREEN_PATHS.Stream,
        },
        MobileGoLive: SCREEN_PATHS.MobileGoLive,
        AVSync: SCREEN_PATHS.AVSync,
        AppReturn: SCREEN_PATHS.AppReturn,
        PopoutChat: SCREEN_PATHS.PopoutChat,
        Embed: SCREEN_PATHS.Embed,
        InfoWidgetEmbed: SCREEN_PATHS.InfoWidgetEmbed,
        LegacyStream: SCREEN_PATHS.LegacyStream,
        DanmuOBS: SCREEN_PATHS.DanmuOBS,
      },
    },
  };

export function getStreamplaceStateFromPath(path: string) {
  const ret = getStateFromPath(path, streamplaceLinkingOptions.config);
  if (!ret) {
    throw new Error(`Invalid path: ${path}`);
  }
  return ret;
}
