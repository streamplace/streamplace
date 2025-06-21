// Web-only provider
import "@rainbow-me/rainbowkit/styles.css";

import { LinkingOptions } from "@react-navigation/native";
import { WalletProvider } from "hooks/useWallet";
import React, { useEffect } from "react";
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
    <WalletProvider>
      <SharedProvider linking={linking}>{children}</SharedProvider>
    </WalletProvider>
  );
}
