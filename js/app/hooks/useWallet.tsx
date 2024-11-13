import { createContext, useContext, useEffect, useState } from "react";
import { createWalletClient, http } from "viem";
import usePlatform from "./usePlatform";
import {
  ConnectButton,
  getDefaultConfig,
  RainbowKitProvider,
} from "@rainbow-me/rainbowkit";
import React from "react";
import { View } from "react-native";
import { privateKeyToAccount } from "viem/accounts";
import { arbitrum, base, mainnet, optimism, polygon } from "viem/chains";
import { Paragraph } from "tamagui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  notActiveWallet,
  WalletStuff,
  SignTypedDataFn,
} from "./useWallet.shared";

const WalletContext = createContext(notActiveWallet);

export default function useWallet(): WalletStuff {
  const { isElectron } = usePlatform();
  if (isElectron) {
    return useViemWallet();
  }
  return useWagmiWallet();
}

export function useViemWallet(): WalletStuff {
  return useContext(WalletContext);
}

export function useWagmiWallet(): WalletStuff {
  return useContext(WalletContext);
}

type ElectronWindow = Window &
  typeof globalThis & {
    getPrivateKey: () => Promise<`0x${string}`>;
  };

const queryClient = new QueryClient();

const defaultConfig = getDefaultConfig({
  appName: "Aquareum",
  appUrl: "https://aquareum.tv",
  projectId: "32c8489fbff0b10e2e011b36c36b4466",
  chains: [mainnet, polygon, optimism, arbitrum, base],
  ssr: true, // If your dApp uses server side rendering (SSR)
});

export function ViemWalletProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  let WalletProvider: React.ElementType = RainbowKitProvider;
  const { isElectron } = usePlatform();
  if (!isElectron) {
    return (
      <View>
        <Paragraph>ViemWalletProvider only supports electron</Paragraph>
      </View>
    );
  }
  const [client, setClient] = useState(notActiveWallet);
  WalletProvider = React.Fragment;
  useEffect(() => {
    (async () => {
      const account = privateKeyToAccount(
        await (window as ElectronWindow).getPrivateKey(),
      );

      const client = createWalletClient({
        account,
        chain: mainnet,
        transport: http(),
      });

      setClient({
        address: account.address.toLowerCase(),
        signTypedData: (async (args) => {
          const signature = await client.signTypedData({
            ...args,
            account: account,
          } as any);
          return signature;
        }) as SignTypedDataFn,
      });
    })();
  }, []);

  if (client === notActiveWallet) {
    return (
      <View>
        <Paragraph>Loading wallet...</Paragraph>
      </View>
    );
  }

  return (
    <WalletContext.Provider value={client}>{children}</WalletContext.Provider>
  );
}

export function WagmiWalletProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  return <>{children}</>;
}

export function WalletProvider({ children }: { children: React.ReactNode }) {
  const { isElectron } = usePlatform();
  const Wallet = isElectron ? ViemWalletProvider : WagmiWalletProvider;
  return (
    <QueryClientProvider client={queryClient}>
      <Wallet>{children}</Wallet>
    </QueryClientProvider>
  );
}

export function ConnectWallet({ children }: { children: React.ReactNode }) {
  const wallet = useWallet();
  const { isElectron } = usePlatform();
  if (!wallet.address) {
    if (isElectron) {
      return (
        <View>
          <Paragraph>
            Fatal error: there should be a local wallet here...
          </Paragraph>
        </View>
      );
    }
    return <ConnectButton />;
  }
  return <>{children}</>;
}
