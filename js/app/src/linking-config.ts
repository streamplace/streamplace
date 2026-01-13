/**
 * Shared linking configuration for react-navigation
 * Used both for URL parsing (inbound) and URL generation (outbound)
 */

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
