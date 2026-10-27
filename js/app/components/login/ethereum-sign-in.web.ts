// Sign in with Ethereum: WalletConnect (Reown AppKit) on the web. The login
// form loads this module with import() so the wallet libraries stay out of
// the main bundle until someone asks for them (see babel-plugin-wallet-libs).
// Typechecked on its own by tsconfig.wallet.json: WalletConnect's typings
// pull in @types/node.
import { createAppKit } from "@reown/appkit";
import { WagmiAdapter } from "@reown/appkit-adapter-wagmi";
import { mainnet } from "@reown/appkit/networks";
import { getAccount, signMessage, watchAccount } from "@wagmi/core";
import { createSiweMessage, generateSiweNonce } from "viem/siwe";
import {
  completeEthereumSignIn,
  type EthereumSignInOptions,
} from "./ethereum-session";

type Account = { address: `0x${string}`; chainId: number };

let kit: {
  projectId: string;
  modal: ReturnType<typeof createAppKit>;
  adapter: WagmiAdapter;
} | null = null;

function getKit(opts: EthereumSignInOptions) {
  if (kit?.projectId === opts.projectId) return kit;
  const adapter = new WagmiAdapter({
    projectId: opts.projectId,
    networks: [mainnet],
  });
  const modal = createAppKit({
    adapters: [adapter],
    networks: [mainnet],
    projectId: opts.projectId,
    metadata: {
      name: opts.siteTitle,
      description: opts.siteTitle,
      url: window.location.origin,
      icons: [],
    },
    themeMode: opts.themeMode,
    themeVariables: {
      "--w3m-accent": opts.accent,
      "--w3m-border-radius-master": "0px",
    },
    features: { analytics: false, email: false, socials: [], swaps: false },
  });
  kit = { projectId: opts.projectId, modal, adapter };
  return kit;
}

// The connected account, opening the wallet picker first if nothing is
// connected; null if the picker is closed without connecting.
function connect({
  modal,
  adapter,
}: NonNullable<typeof kit>): Promise<Account | null> {
  const config = adapter.wagmiConfig;
  const current = getAccount(config);
  if (current.address && current.chainId) {
    return Promise.resolve({
      address: current.address,
      chainId: current.chainId,
    });
  }
  return new Promise((resolve) => {
    const stop = () => {
      unwatch();
      unsubscribe();
    };
    const unwatch = watchAccount(config, {
      onChange(account) {
        if (!account.address || !account.chainId) return;
        stop();
        modal.close();
        resolve({ address: account.address, chainId: account.chainId });
      },
    });
    let opened = false;
    const unsubscribe = modal.subscribeState((state) => {
      if (state.open) opened = true;
      if (state.open || !opened) return;
      stop();
      resolve(null);
    });
    modal.open({ view: "Connect", namespace: "eip155" });
  });
}

// Signs in with the user's wallet; returns without signing in if they close
// the wallet picker.
export async function signInWithEthereum(
  opts: EthereumSignInOptions,
): Promise<void> {
  const k = getKit(opts);
  const account = await connect(k);
  if (!account) return;
  const { address, chainId } = account;
  const message = createSiweMessage({
    address,
    chainId,
    domain: window.location.host,
    uri: window.location.origin,
    version: "1",
    // a local nonce until the entryway issues one with its sign-in backend
    nonce: generateSiweNonce(),
    statement: `Sign in to ${opts.siteTitle}`,
    issuedAt: new Date(),
  });
  const signature = await signMessage(k.adapter.wagmiConfig, {
    account: address,
    message,
  });
  await completeEthereumSignIn({ message, signature, address, chainId });
}
