export interface EthereumSignInProof {
  message: string;
  signature: `0x${string}`;
  address: `0x${string}`;
  chainId: number;
}

/** An injected wallet's EIP-1193 provider. */
export interface Eip1193Provider {
  request(args: { method: string; params?: unknown[] }): Promise<any>;
}

// The backend that turns a signed SIWE message into a session on the
// entryway lands here; until then the login form shows this error.
export async function completeEthereumSignIn(
  _proof: EthereumSignInProof,
): Promise<void> {
  throw new Error("Sign in with Ethereum isn't available on this server yet.");
}
