// Web-only provider
import "@rainbow-me/rainbowkit/styles.css";

import { LinkingOptions } from "@react-navigation/native";
import SharedProvider from "./provider.shared";
import React from "react";
import { WalletProvider } from "hooks/useWallet";
import OAuthClient from "atproto/oauth";
import { AuthProvider } from "hooks/useAuthContext";

export default function Provider({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  return (
    <SharedProvider linking={linking}>
      <AuthProvider client={OAuthClient}>
        <WalletProvider>{children}</WalletProvider>
      </AuthProvider>
    </SharedProvider>
  );
}
