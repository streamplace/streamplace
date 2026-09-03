/**
 * Shared linking configuration for react-navigation
 * Used both for URL parsing (inbound) and URL generation (outbound)
 */

import {
  getStateFromPath,
  LinkingOptions,
  NavigationState,
  PartialState,
} from "@react-navigation/native";
import * as ExpoLinking from "expo-linking";

const AT_URI_PREFIX = "at://";

export function parseAtUriPath(path: string): {
  authority: string;
  collection: string;
  rkey: string;
} | null {
  const cleanPath = path.startsWith("/") ? path.slice(1) : path;
  if (!cleanPath.startsWith(AT_URI_PREFIX)) {
    return null;
  }
  const rest = cleanPath.slice(AT_URI_PREFIX.length);
  const parts = rest.split("/");
  const authority = parts[0];
  if (!authority) return null;
  return {
    authority,
    collection: parts[1] ?? "",
    rkey: parts[2] ?? "",
  };
}

function resolveAtUriNavigation(
  path: string,
): PartialState<NavigationState> | null {
  const atUri = parseAtUriPath(path);
  if (!atUri) return null;
  // if just authority, redirect to stream page
  if (!atUri.collection) {
    return {
      routes: [
        {
          name: "Stream",
          params: { user: atUri.authority },
        },
      ],
    };
  } else if (atUri.collection === "place.stream.livestream") {
    // livestream isn't historical so just redirect to user
    return {
      routes: [
        {
          name: "Stream",
          params: { user: atUri.authority },
        },
      ],
    };
  } else if (atUri.collection === "place.stream.video" && atUri.rkey) {
    // A video record: redirect to the canonical VOD player URL
    // (/<authority>/video/<rkey>). react-navigation rewrites the address bar
    // to that path.
    return {
      routes: [
        {
          name: "Video",
          params: { user: atUri.authority, tid: atUri.rkey },
        },
      ],
    };
  } else {
    // otherwise, redirect to home page and do a 'could not find'
    return {
      routes: [
        {
          name: "MainTabs",
          state: {
            routes: [
              {
                name: "HomeTab",
                state: {
                  routes: [
                    {
                      name: "HomeMain",
                    },
                  ],
                },
              },
            ],
          },
        },
      ],
    };
  }
}

export const SCREEN_PATHS = {
  // HomeTab screens
  HomeMain: "",
  About: "about",
  Download: "download",
  LiveDashboard: "live",
  Login: "login",
  Multi: "multi/:config",
  Support: "support",
  Upload: "upload",
  UploadVideo: "upload/video/:tid",
  UploadDrafts: "upload/drafts",
  UploadLivestreams: "upload/livestreams",
  UploadVideos: "upload/videos",
  VideoList: "video",
  UserVideoList: ":did/video",
  // Settings screens
  MainSettings: "settings",
  AboutCategory: "settings/about",
  AccountCategory: "settings/account",
  BackupSettings: "/settings/streaming/backup",
  StreamingCategory: "settings/streaming",
  WebhooksSettings: "settings/streaming/webhooks",
  RecommendationsSettings: "settings/streaming/recommendations",
  PrivacyCategory: "settings/privacy",
  NotificationsCategory: "settings/notifications",
  DanmuCategory: "settings/danmu",
  AdvancedCategory: "settings/advanced",
  DeveloperSettings: "settings/developer",
  MultistreamCategory: "settings/streaming/multistream",
  KeyManagement: "settings/streaming/key-management",
  LanguagesCategory: "settings/languages",
  BrandingAdmin: "settings/branding",
  AccessAdmin: "settings/access",
  BadgeSelection: "settings/account/badges",
  BadgeIssuer: "settings/issue-badges",
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
  DanmuOBS: "widgets/:user/danmu",
  PopoutStreamMonitor: "widgets/stream-monitor",
  PopoutInfoWidget: "widgets/info",
  PopoutMultistream: "widgets/multistream",
  PopoutLivestream: "widgets/livestream",
  Video: ":user/video/:tid",
  Vod: ":user/minimal-vod/:tid",
  VodEmbed: "embed/:user/video/:tid",
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
    getStateFromPath: (path, options) => {
      const atUriState = resolveAtUriNavigation(path);
      if (atUriState) return atUriState;
      return getStateFromPath(path, options);
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
                Upload: SCREEN_PATHS.Upload,
                UploadVideo: SCREEN_PATHS.UploadVideo,
                UploadDrafts: SCREEN_PATHS.UploadDrafts,
                UploadLivestreams: SCREEN_PATHS.UploadLivestreams,
                UploadVideos: SCREEN_PATHS.UploadVideos,
              },
            },
            VideosTab: {
              screens: {
                VideoList: SCREEN_PATHS.VideoList,
                UserVideoList: SCREEN_PATHS.UserVideoList,
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
                NotificationsCategory: SCREEN_PATHS.NotificationsCategory,
                DanmuCategory: SCREEN_PATHS.DanmuCategory,
                AdvancedCategory: SCREEN_PATHS.AdvancedCategory,
                MultistreamCategory: SCREEN_PATHS.MultistreamCategory,
                KeyManagement: SCREEN_PATHS.KeyManagement,
                LanguagesCategory: SCREEN_PATHS.LanguagesCategory,
                BrandingAdmin: SCREEN_PATHS.BrandingAdmin,
                AccessAdmin: SCREEN_PATHS.AccessAdmin,
                BadgeSelection: SCREEN_PATHS.BadgeSelection,
                BadgeIssuer: SCREEN_PATHS.BadgeIssuer,
              },
            },
          },
        },
        // Root stack screens (outside tabs - full-screen experiences)
        Stream: {
          path: SCREEN_PATHS.Stream,
        },
        MobileGoLive: SCREEN_PATHS.MobileGoLive,
        AppReturn: SCREEN_PATHS.AppReturn,
        PopoutChat: SCREEN_PATHS.PopoutChat,
        Embed: SCREEN_PATHS.Embed,
        InfoWidgetEmbed: SCREEN_PATHS.InfoWidgetEmbed,
        DanmuOBS: SCREEN_PATHS.DanmuOBS,
        PopoutStreamMonitor: SCREEN_PATHS.PopoutStreamMonitor,
        PopoutInfoWidget: SCREEN_PATHS.PopoutInfoWidget,
        PopoutMultistream: SCREEN_PATHS.PopoutMultistream,
        PopoutLivestream: SCREEN_PATHS.PopoutLivestream,
        Video: SCREEN_PATHS.Video,
        Vod: SCREEN_PATHS.Vod,
        VodEmbed: SCREEN_PATHS.VodEmbed,
      },
    },
  };

export function getStreamplaceStateFromPath(path: string) {
  const atUriState = resolveAtUriNavigation(path);
  if (atUriState) return atUriState;
  const ret = getStateFromPath(path, streamplaceLinkingOptions.config);
  if (!ret) {
    throw new Error(`Invalid path: ${path}`);
  }
  return ret;
}
