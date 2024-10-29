// Web-only provider
import "@rainbow-me/rainbowkit/styles.css";

import { getDefaultConfig, RainbowKitProvider } from "@rainbow-me/rainbowkit";
import { WagmiProvider } from "wagmi";
import { mainnet, polygon, optimism, arbitrum, base } from "wagmi/chains";
import { QueryClientProvider, QueryClient } from "@tanstack/react-query";
import { View, Text } from "tamagui";
import SharedProvider from "./provider.shared";
import { LinkingOptions } from "@react-navigation/native";

const queryClient = new QueryClient();

const config = getDefaultConfig({
  appName: "Aquareum",
  appUrl: "https://aquareum.tv",
  projectId: "32c8489fbff0b10e2e011b36c36b4466",
  chains: [mainnet, polygon, optimism, arbitrum, base],
  ssr: true, // If your dApp uses server side rendering (SSR)
});

export default function Provider({
  children,
  linking,
}: {
  children: React.ReactNode;
  linking: LinkingOptions<ReactNavigation.RootParamList>;
}) {
  return (
    <SharedProvider linking={linking}>
      <WagmiProvider config={config}>
        <QueryClientProvider client={queryClient}>
          <RainbowKitProvider coolMode={true}>
            {/* RainbowKitProvider hides our children unless we do this...? */}
            <View
              id="rainbowkit-interior" // Also this......?????
              f={1}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100vw",
                height: "100vh",
              }}
            >
              {children}
            </View>
          </RainbowKitProvider>
        </QueryClientProvider>
      </WagmiProvider>
    </SharedProvider>
  );
}
