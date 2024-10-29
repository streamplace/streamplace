import React from "react";
import SharedProvider from "./provider.shared";
import { LinkingOptions } from "@react-navigation/native";

export default function Provider({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  return <SharedProvider linking={linking}>{children}</SharedProvider>;
}
