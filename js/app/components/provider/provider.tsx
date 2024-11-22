// Web-only provider
import "@rainbow-me/rainbowkit/styles.css";

import { LinkingOptions } from "@react-navigation/native";
import SharedProvider from "./provider.shared";
import React from "react";
import { WalletProvider } from "hooks/useWallet";

export default function Provider({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  return (
    <WalletProvider>
      <SharedProvider linking={linking}>{children}</SharedProvider>
    </WalletProvider>
  );
}
