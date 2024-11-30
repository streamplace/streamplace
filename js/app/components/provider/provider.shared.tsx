import {
  DarkTheme,
  LinkingOptions,
  NavigationContainer,
} from "@react-navigation/native";
import { ToastProvider, ToastViewport } from "@tamagui/toast";
import { useFonts } from "expo-font";
import AquareumProvider from "features/aquareum/aquareumProvider";
import React from "react";
import { PortalProvider, TamaguiProvider } from "tamagui";
import config from "tamagui.config";
import { CurrentToast } from "./CurrentToast";
import { Provider as ReduxProvider } from "react-redux";
import BlueskyProvider from "features/bluesky/blueskyProvider";
import { store } from "store/store";
export default function Provider({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  return (
    <TamaguiProvider config={config} defaultTheme={"dark"}>
      <NavigationContainer theme={DarkTheme} linking={linking}>
        <ReduxProvider store={store}>
          <AquareumProvider>
            <BlueskyProvider>
              <PortalProvider>
                <ToastProvider
                  swipeDirection="vertical"
                  duration={6000}
                  native={
                    [
                      /* uncomment the next line to do native toasts on mobile. NOTE: it'll require you making a dev build and won't work with Expo Go */
                      // 'mobile'
                    ]
                  }
                >
                  <FontProvider>{children}</FontProvider>
                  <CurrentToast />
                  <ToastViewport name="default" top="$8" left={0} right={0} />
                </ToastProvider>
              </PortalProvider>
            </BlueskyProvider>
          </AquareumProvider>
        </ReduxProvider>
      </NavigationContainer>
    </TamaguiProvider>
  );
}

export const FontProvider = ({ children }: { children: React.ReactNode }) => {
  const [fontLoaded, fontError] = useFonts({
    "FiraCode-Light": require("../../assets/fonts/FiraCode-Light.ttf"),
    "FiraCode-Medium": require("../../assets/fonts/FiraCode-Medium.ttf"),
    "FiraCode-Bold": require("../../assets/fonts/FiraCode-Bold.ttf"),
    "FiraSans-Medium": require("../../assets/fonts/FiraSans-Medium.ttf"),
  });

  if (!fontLoaded && !fontError) {
    return null;
  }

  return <>{children}</>;
};
