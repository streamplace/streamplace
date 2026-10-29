import type { Eip1193Provider } from "./ethereum-session";

// Sign in with Ethereum is web-only for now (ethereum-sign-in.web.ts); the
// login form hides it elsewhere. Keeping the WalletConnect code in the .web
// file also keeps its Node typings out of the app's tsc program (see
// tsconfig.wallet.json).
const webOnly = () =>
  Promise.reject(
    new Error("Sign in with Ethereum is only available on the web."),
  );

export const signInWithInjected = (
  _provider: Eip1193Provider,
  _siteTitle: string,
  _signal: AbortSignal,
): Promise<void> => webOnly();

export const signInWithWalletConnect = (_opts: {
  projectId: string;
  siteTitle: string;
  onUri: (uri: string) => void;
  signal: AbortSignal;
}): Promise<void> => webOnly();
