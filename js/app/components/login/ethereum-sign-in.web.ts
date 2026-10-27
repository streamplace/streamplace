// Sign in with Ethereum on the web, with a browser (injected) wallet or
// WalletConnect. The login form loads this module with import() so the
// WalletConnect client stays out of the main bundle until someone asks for
// it. Typechecked on its own by tsconfig.wallet.json: WalletConnect's
// typings pull in @types/node.
import SignClient from "@walletconnect/sign-client";
import { createSiweMessage, generateSiweNonce } from "viem/siwe";
import { getAddress, stringToHex } from "viem/utils";
import {
  completeEthereumSignIn,
  type Eip1193Provider,
} from "./ethereum-session";

async function signIn(
  account: string,
  chainId: number,
  siteTitle: string,
  personalSign: (hexMessage: string, address: string) => Promise<string>,
) {
  const address = getAddress(account);
  const message = createSiweMessage({
    address,
    chainId,
    domain: window.location.host,
    uri: window.location.origin,
    version: "1",
    // a local nonce until the entryway issues one with its sign-in backend
    nonce: generateSiweNonce(),
    statement: `Sign in to ${siteTitle}`,
    issuedAt: new Date(),
  });
  const signature = (await personalSign(
    stringToHex(message),
    address,
  )) as `0x${string}`;
  await completeEthereumSignIn({ message, signature, address, chainId });
}

export async function signInWithInjected(
  provider: Eip1193Provider,
  siteTitle: string,
) {
  const [account] = await provider.request({ method: "eth_requestAccounts" });
  const chainId = Number(await provider.request({ method: "eth_chainId" }));
  await signIn(account, chainId, siteTitle, (hex, address) =>
    provider.request({ method: "personal_sign", params: [hex, address] }),
  );
}

let client: { projectId: string; client: Promise<SignClient> } | null = null;

function getClient(projectId: string, siteTitle: string) {
  if (client?.projectId !== projectId) {
    client = {
      projectId,
      client: SignClient.init({
        projectId,
        metadata: {
          name: siteTitle,
          description: siteTitle,
          url: window.location.origin,
          icons: [],
        },
        // no usage events to WalletConnect's analytics (pulse)
        telemetryEnabled: false,
      }),
    };
  }
  return client.client;
}

// Pairs with a wallet app over WalletConnect: onUri gets the pairing link to
// show as a QR code (or open on a phone), and once the wallet approves it
// signs the SIWE message there. An aborted signal stops before signing.
export async function signInWithWalletConnect(opts: {
  projectId: string;
  siteTitle: string;
  onUri: (uri: string) => void;
  signal: AbortSignal;
}) {
  // Reaching the relay: a bad project id or a blocked network would
  // otherwise leave it retrying, and the button spinning, for good.
  const pairing = (async () => {
    const c = await getClient(opts.projectId, opts.siteTitle);
    const proposal = await c.connect({
      optionalNamespaces: {
        eip155: {
          chains: ["eip155:1"],
          methods: ["personal_sign"],
          events: [],
        },
      },
    });
    return { c, ...proposal };
  })();
  const { c, uri, approval } = await Promise.race([
    pairing,
    new Promise<never>((_, reject) =>
      setTimeout(
        () => reject(new Error("Couldn't reach WalletConnect.")),
        15_000,
      ),
    ),
  ]);
  if (uri) opts.onUri(uri);
  const session = await approval();
  try {
    if (opts.signal.aborted) return;
    // accounts are CAIP-10: eip155:<chain>:<address>
    const [, chain, account] = session.namespaces.eip155.accounts[0].split(":");
    await signIn(account, Number(chain), opts.siteTitle, (hex, address) =>
      c.request({
        topic: session.topic,
        chainId: `eip155:${chain}`,
        request: { method: "personal_sign", params: [hex, address] },
      }),
    );
  } finally {
    // a sign-in needs one signature, not a standing session in the wallet
    c.disconnect({
      topic: session.topic,
      reason: { code: 6000, message: "User disconnected." },
    }).catch(() => {});
  }
}
