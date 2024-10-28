import { DarkTheme, NavigationContainer } from "@react-navigation/native";
import { ToastProvider, ToastViewport } from "@tamagui/toast";
import { AquareumProvider } from "hooks/useAquareumNode";
import React from "react";
import { PortalProvider, TamaguiProvider } from "tamagui";
import config from "tamagui.config";
import { CurrentToast } from "./CurrentToast";

export default function Provider({ children }: { children: React.ReactNode }) {
  return (
    <TamaguiProvider config={config} defaultTheme={"dark"}>
      <NavigationContainer theme={DarkTheme}>
        <AquareumProvider>
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
              {children}
              <CurrentToast />
              <ToastViewport name="default" top="$8" left={0} right={0} />
            </ToastProvider>
          </PortalProvider>
        </AquareumProvider>
      </NavigationContainer>
    </TamaguiProvider>
  );
}
