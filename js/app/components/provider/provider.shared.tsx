import {
  DarkTheme,
  LinkingOptions,
  NavigationContainer,
} from "@react-navigation/native";
import * as Sentry from "@sentry/react-native";
import {
  BrandedThemeProvider,
  I18nProvider,
  ThemeProvider,
  StreamplaceProvider as ZustandStreamplaceProvider,
} from "@streamplace/components";
import { useFonts } from "expo-font";
import BlueskyProvider from "features/bluesky/blueskyProvider";
import { SessionBrokerProvider } from "features/session-broker/provider";
import StreamplaceProvider from "features/streamplace/streamplaceProvider";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import React from "react";
import { useStore } from "store";
import { useOAuthSession } from "store/hooks";

import { i18n, initI18next } from "@streamplace/components";
import * as Application from "expo-application";
import Constants from "expo-constants";
import * as Updates from "expo-updates";
import { Platform } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";

// Initialize the shared i18next instance: registers the React/Fluent/backend
// plugins and loads the stored locale. Without this, the `i18n` instance
// passed to I18nProvider below stays uninitialized and nothing translates.
void initI18next();

// get proper DSN for environment
// on ios/android it's process.env.EXPO_PUBLIC_SENTRY_DSN
// on web it will be injected at runtime
let dsn = undefined;
if (Platform.OS === "web") {
  dsn = (window as any).SENTRY_DSN;
} else {
  dsn = process.env.EXPO_PUBLIC_SENTRY_DSN || undefined;
}

if (dsn) {
  Sentry.setExtras({
    manifest: Updates.manifest,
    linkingUri: Constants.linkingUri,
  });
  Sentry.setTag("expoChannel", Updates.channel);
  Sentry.setTag("appVersion", Application.nativeApplicationVersion);
  Sentry.setTag("deviceId", Constants.sessionId);
  Sentry.setTag("executionEnvironment", Constants.executionEnvironment);
  Sentry.setTag("expoGoVersion", Constants.expoVersion);
  Sentry.setTag("expoRuntimeVersion", Constants.expoRuntimeVersion);
}

const isWeb = Platform.OS === "web";

// set transparent dark theme on web for easier OBS browser sourcing
const darkTheme = isWeb ? { background: "transparent" } : {};

const SPDarkTheme = {
  ...DarkTheme,
  colors: {
    ...DarkTheme.colors,
    ...darkTheme,
  },
};

function ProviderInner({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  if (dsn) {
    Sentry.init({
      dsn,
      // Adds more context data to events (IP address, cookies, user, etc.)
      // For more information, visit: https://docs.sentry.io/platforms/react-native/data-management/data-collected/
      sendDefaultPii: true,

      // Configure Session Replay
      replaysSessionSampleRate: 0.1,
      replaysOnErrorSampleRate: 1,
      integrations: [
        Sentry.mobileReplayIntegration(),
        Sentry.feedbackIntegration(),
      ],

      // uncomment the line below to enable Spotlight (https://spotlightjs.com)
      spotlight: __DEV__,
    });
  }

  return (
    <SafeAreaProvider>
      <NavigationContainer theme={SPDarkTheme} linking={linking}>
        <ThemeProvider forcedTheme="dark">
          <I18nProvider i18n={i18n}>
            <StreamplaceProvider>
              <BlueskyProvider>
                <NewStreamplaceProvider>
                  <SessionBrokerProvider />
                  <BrandedThemeProvider forcedTheme="dark">
                    <FontProvider>{children}</FontProvider>
                  </BrandedThemeProvider>
                </NewStreamplaceProvider>
              </BlueskyProvider>
            </StreamplaceProvider>
          </I18nProvider>
        </ThemeProvider>
      </NavigationContainer>
    </SafeAreaProvider>
  );
}

export const NewStreamplaceProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const { url } = useStreamplaceNode();
  const oauthSession = useOAuthSession();
  const openLoginModal = useStore((s) => s.openLoginModal);
  return (
    <ZustandStreamplaceProvider
      url={url}
      oauthSession={oauthSession}
      onNeedsLogin={openLoginModal ? () => openLoginModal() : undefined}
    >
      {children}
    </ZustandStreamplaceProvider>
  );
};

export const FontProvider = ({ children }: { children: React.ReactNode }) => {
  const [fontLoaded, fontError] = useFonts({
    // Geist (Sans Serif) — the design system uses exactly three weights
    "Geist-Regular": require("../../assets/fonts/Geist-Regular.ttf"),
    "Geist-Medium": require("../../assets/fonts/Geist-Medium.ttf"),
    "Geist-SemiBold": require("../../assets/fonts/Geist-SemiBold.ttf"),

    // Geist Mono — stream keys, ingest URLs, timers
    "GeistMono-Regular": require("../../assets/fonts/GeistMono-Regular.ttf"),
    "GeistMono-Medium": require("../../assets/fonts/GeistMono-Medium.ttf"),
    "GeistMono-SemiBold": require("../../assets/fonts/GeistMono-SemiBold.ttf"),

    // Inter — the alternative sans a node can pick with the typeface
    // branding key (same three weights, mono stays Geist Mono)
    "Inter-Regular": require("@expo-google-fonts/inter/400Regular/Inter_400Regular.ttf"),
    "Inter-Medium": require("@expo-google-fonts/inter/500Medium/Inter_500Medium.ttf"),
    "Inter-SemiBold": require("@expo-google-fonts/inter/600SemiBold/Inter_600SemiBold.ttf"),
  });

  if (!fontLoaded && !fontError) {
    return null;
  }

  return <>{children}</>;
};

let wrappedProvider = ProviderInner;
if (dsn) {
  wrappedProvider = Sentry.wrap(ProviderInner) as any;
}

export default wrappedProvider;
