import { TypedData, SignTypedDataParameters } from "viem";

export type SignTypedDataFn = <
  const typedData extends TypedData | { [key: string]: unknown },
  primaryType extends string,
>(
  args: Omit<SignTypedDataParameters<typedData, primaryType, any>, "account">,
) => Promise<string>;

export type WalletStuff = {
  address?: string;
  signTypedData: SignTypedDataFn;
};

export const notActive = (async (args) => {
  throw new Error("Wallet not active");
}) as SignTypedDataFn;
export const notActiveWallet: WalletStuff = {
  address: undefined,
  signTypedData: notActive,
};
