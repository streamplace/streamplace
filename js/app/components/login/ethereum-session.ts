export interface EthereumSignInProof {
  message: string;
  signature: `0x${string}`;
  address: `0x${string}`;
  chainId: number;
}

export interface EthereumSignInOptions {
  projectId: string;
  siteTitle: string;
  themeMode: "dark" | "light";
  accent: string;
}

// The backend that turns a signed SIWE message into a session on the
// entryway lands here; until then the login form shows this error.
export async function completeEthereumSignIn(
  _proof: EthereumSignInProof,
): Promise<void> {
  throw new Error("Sign in with Ethereum isn't available on this server yet.");
}
