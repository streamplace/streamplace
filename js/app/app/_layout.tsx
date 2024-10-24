import background from "./background";
import "../tamagui-web.css";
import { Link } from "expo-router";
import {
  Anchor,
  Button,
  useTheme,
  Text,
  styled,
  isWeb,
  View,
  H4,
} from "tamagui";

import { useEffect } from "react";
import { useColorScheme } from "hooks/useColorScheme";
import {
  DarkTheme,
  DefaultTheme,
  ThemeProvider,
} from "@react-navigation/native";
import { useFonts } from "expo-font";
import { SplashScreen, Stack } from "expo-router";
import { Provider } from "components";
import { Helmet } from "react-native-helmet-async";
import { Settings } from "@tamagui/lucide-icons";
import { topSafeHeight } from "./platform";
import { SafeAreaView } from "react-native";
import usePlatform from "hooks/usePlatform";

export {
  // Catch any errors thrown by the Layout component.
  ErrorBoundary,
} from "expo-router";

export const unstable_settings = {
  // Ensure that reloading on `/modal` keeps a back button present.
  initialRouteName: "(tabs)",
};

// Prevent the splash screen from auto-hiding before asset loading is complete.
SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const [fontLoaded, fontError] = useFonts({
    "FiraCode-Light": require("../assets/fonts/FiraCode-Light.ttf"),
    "FiraCode-Medium": require("../assets/fonts/FiraCode-Medium.ttf"),
    "FiraCode-Bold": require("../assets/fonts/FiraCode-Bold.ttf"),
    "FiraSans-Medium": require("../assets/fonts/FiraSans-Medium.ttf"),
  });

  useEffect(() => {
    if (fontLoaded || fontError) {
      // Hide the splash screen after the fonts have loaded (or an error was returned) and the UI is ready.
      SplashScreen.hideAsync();
    }
  }, [fontLoaded, fontError]);

  if (!fontLoaded && !fontError) {
    return null;
  }

  return <RootLayoutNav />;
}

export const LinkNoUnderline = styled(Link, {});

// hack to prevent an error we can't do anything about in
// HLS.js from popping up a full screen error page in dev
const IGNORE_THIS_ERROR =
  "The fetching process for the media resource was aborted by the user agent at the user's request.";
if (isWeb && typeof window !== "undefined") {
  const handler = (e: PromiseRejectionEvent) => {
    if (`${e.reason}`.includes(IGNORE_THIS_ERROR)) {
      e.preventDefault();
      e.stopPropagation();
      e.stopImmediatePropagation();
      console.error(e);
    }
  };
  window.addEventListener("unhandledrejection", handler);
}

function RootLayoutNav() {
  const colorScheme = useColorScheme();

  useEffect(() => {
    background();
  }, []);

  return (
    <Provider>
      <View f={1} paddingTop={topSafeHeight()} backgroundColor="$background">
        <SafeAreaView style={{ flex: 1 }}>
          <ThemeProvider
            value={colorScheme === "dark" ? DarkTheme : DefaultTheme}
          >
            {isWeb && (
              <Helmet>
                <title>Aquareum</title>
              </Helmet>
            )}
            <Stack>
              <Stack.Screen
                name="(tabs)"
                options={{
                  title: "",
                  headerShown: true,
                  headerRight: () => {
                    return (
                      <Link href="/settings" asChild>
                        <Button icon={<Settings size="$2" />}></Button>
                      </Link>
                    );
                  },
                  headerLeft: () => (
                    <Anchor href="https://explorer.livepeer.org/treasury/74518185892381909671177921640414850443801430499809418110611019961553289709442">
                      <View bg="$accentColor" br="$5" padding="$2">
                        <H4 fontSize="$4">What's Aquareum?</H4>
                      </View>
                    </Anchor>
                  ),
                }}
              />
              <Stack.Screen
                name="embed/[stream]"
                options={{
                  headerShown: false,
                }}
              />
              <Stack.Screen
                name="settings"
                options={{
                  title: "Settings",
                  presentation: "modal",
                  animation: "slide_from_right",
                  gestureEnabled: true,
                  gestureDirection: "horizontal",
                }}
              />
            </Stack>
          </ThemeProvider>
        </SafeAreaView>
      </View>
    </Provider>
  );
}
