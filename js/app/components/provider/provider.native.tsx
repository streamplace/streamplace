// this comment is here to stop auto-alphabetizing imports lol
import { LinkingOptions } from "@react-navigation/native";
import React from "react";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import SharedProvider from "./provider.shared";

export default function Provider({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SharedProvider linking={linking}>{children}</SharedProvider>
    </GestureHandlerRootView>
  );
}
