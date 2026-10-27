import type { EthereumSignInOptions } from "./ethereum-session";

// Sign in with Ethereum is web-only for now (ethereum-sign-in.web.ts); the
// login form hides it elsewhere. Keeping the WalletConnect code in the .web
// file also keeps its Node typings out of the app's tsc program (see
// tsconfig.wallet.json).
export async function signInWithEthereum(
  _opts: EthereumSignInOptions,
): Promise<void> {
  throw new Error("Sign in with Ethereum is only available on the web.");
}
