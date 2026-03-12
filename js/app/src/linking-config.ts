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
    // getPathFromState(state, options): string {
    //   const routes = state.routes;
    //   console.log("getPathFromState", JSON.stringify({ routes }, null, 2));
    //   // if (!route.path) {
    //   //   return "";
    //   // }
    //   return "";
    // },
    prefixes: [ExpoLinking.createURL("")],
    getStateFromPath: (path, config) => {
      console.log("getStateFromPath", JSON.stringify({ path, config }));
      // Use default parsing
      const state = getStateFromPath(path, config);
      console.log("getStateFromPath returning", state);

      // if (state) {
      //   // Check if we're navigating to a settings detail screen
      //   const routes = state.routes;
      //   const mainTabsRoute = routes.find((r: any) => r.name === "MainTabs");

      //   if (mainTabsRoute?.state) {
      //     const settingsTabRoute = mainTabsRoute.state.routes?.find(
      //       (r: any) => r.name === "SettingsTab",
      //     );

      //     // if we're going to a settings detail screen, but MainSettings is not in the stack,
      //     // we need to insert it at the bottom of the stack so we can escape to MainSettings
      //     if (settingsTabRoute?.state) {
      //       const settingsStack = settingsTabRoute.state.routes;
      //       const firstRoute = settingsStack?.[0];
      //       if (
      //         firstRoute?.name !== "MainSettings" &&
      //         settingsStack?.length === 1
      //       ) {
      //         return {
      //           ...state,
      //           routes: state.routes.map((r: any) => {
      //             if (r.name === "MainTabs" && r.state) {
      //               return {
      //                 ...r,
      //                 state: {
      //                   ...r.state,
      //                   routes: r.state.routes.map((tabRoute: any) => {
      //                     if (tabRoute.name === "SettingsTab" && tabRoute.state) {
      //                       return {
      //                         ...tabRoute,
      //                         state: {
      //                           ...tabRoute.state,
      //                           routes: [
      //                             { name: "MainSettings" },
      //                             ...tabRoute.state.routes,
      //                           ],
      //                         },
      //                       };
      //                     }
      //                     return tabRoute;
      //                   }),
      //                 },
      //               };
      //             }
      //             return r;
      //           }),
      //         };
      //       }
      //     }
      //   }
      // }

      return state;
    },
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
