import { LinkingOptions } from "@react-navigation/native";
import React, { useEffect } from "react";
import { SafeAreaProvider } from "react-native-safe-area-context";
import SharedProvider from "./provider.shared";

export default function Provider({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  useEffect(() => {
    // atproto requires 127.0.0.1 rather than localhost
    const u = new URL(document.location.href);
    if (u.hostname === "localhost") {
      u.hostname = "127.0.0.1";
      document.location.href = u.toString();
    }
  }, []);
  return (
    <SafeAreaProvider>
      <SharedProvider linking={linking}>{children}</SharedProvider>
    </SafeAreaProvider>
  );
}
