import { notActiveWallet, WalletStuff } from "./useWallet.shared";

export default function useWallet(): WalletStuff {
  return notActiveWallet;
}
