import {
  Button,
  Loader,
  Text,
  useSiteTitle,
  useTheme,
  useTranslation,
  zero,
} from "@streamplace/components";
import { Image } from "expo-image";
import { useEffect, useState } from "react";
import { Linking, Pressable, View } from "react-native";
import type { Eip1193Provider } from "./ethereum-session";
import { LoginQr } from "./login-qr";
import { OrDivider } from "./or-divider";

const QR_SIZE = 200;

// EIP-6963: each browser wallet announces itself with a name and icon.
interface InjectedWallet {
  info: { uuid: string; name: string; icon: string; rdns: string };
  provider: Eip1193Provider;
}

// The browser wallets on this page: those announced over EIP-6963, else the
// one at window.ethereum (wallets from before EIP-6963).
function useInjectedWallets(fallbackName: string) {
  const [wallets, setWallets] = useState<InjectedWallet[]>([]);
  useEffect(() => {
    const found = new Map<string, InjectedWallet>();
    const onAnnounce = (e: Event) => {
      const wallet = (e as CustomEvent<InjectedWallet>).detail;
      found.set(wallet.info.uuid, wallet);
      setWallets([...found.values()]);
    };
    window.addEventListener("eip6963:announceProvider", onAnnounce);
    window.dispatchEvent(new Event("eip6963:requestProvider"));
    const legacy = (window as { ethereum?: Eip1193Provider }).ethereum;
    if (!found.size && legacy) {
      setWallets([
        {
          info: {
            uuid: "injected",
            name: fallbackName,
            icon: "",
            rdns: "injected",
          },
          provider: legacy,
        },
      ]);
    }
    return () =>
      window.removeEventListener("eip6963:announceProvider", onAnnounce);
  }, [fallbackName]);
  return wallets;
}

// The user turned the request down in their wallet: EIP-1193's 4001, or
// WalletConnect's 5000 for a rejected session.
const isRejection = (e: any) => e?.code === 4001 || e?.code === 5000;
const message = (e: any) => e?.message ?? String(e);

// Sign in with Ethereum (web only): a WalletConnect QR code, shown at once
// when the node has a project id for it, then the browser's own wallets.
export function EthereumWallets({
  walletConnectProjectId,
  onBack,
}: {
  walletConnectProjectId?: string;
  onBack: () => void;
}) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const siteTitle = useSiteTitle();
  const wallets = useInjectedWallets(t("login-browser-wallet"));
  const [busy, setBusy] = useState<string | null>(null);
  const [uri, setUri] = useState<string | null>(null);
  const [qrError, setQrError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // aborted when the user leaves the list (Back unmounts it), so a wallet
  // still connecting then doesn't go on to ask for a signature
  const [left] = useState(() => new AbortController());
  useEffect(() => () => left.abort(), [left]);
  const square = { borderRadius: theme.borderRadius.none };

  // pair with a wallet app as soon as the panel opens
  useEffect(() => {
    if (!walletConnectProjectId) return;
    import("./ethereum-sign-in")
      .then((eth) =>
        eth.signInWithWalletConnect({
          projectId: walletConnectProjectId,
          siteTitle,
          onUri: setUri,
          signal: left.signal,
        }),
      )
      .catch((e) => {
        if (!left.signal.aborted) setQrError(message(e));
      });
  }, [walletConnectProjectId, siteTitle, left]);

  const connectInjected = async ({ info, provider }: InjectedWallet) => {
    setError(null);
    setBusy(info.uuid);
    try {
      const eth = await import("./ethereum-sign-in");
      await eth.signInWithInjected(provider, siteTitle, left.signal);
    } catch (e: any) {
      if (!isRejection(e)) setError(message(e));
    } finally {
      setBusy(null);
    }
  };

  return (
    <View style={zero.gap.all[4]}>
      {walletConnectProjectId && (
        <>
          <View style={[zero.layout.flex.alignCenter, zero.gap.all[3]]}>
            <Text size="sm" color="muted">
              {t("login-walletconnect-scan")}
            </Text>
            <View
              style={[
                zero.layout.flex.alignCenter,
                zero.layout.flex.justifyCenter,
                { width: QR_SIZE, height: QR_SIZE },
              ]}
            >
              {qrError ? (
                <Text
                  size="sm"
                  color={theme.colors.danger}
                  style={{ textAlign: "center" }}
                >
                  {qrError}
                </Text>
              ) : uri ? (
                // on a phone the QR can't scan itself: tapping it opens the
                // wallet app instead
                <Pressable
                  testID="login-walletconnect-qr"
                  accessibilityRole="link"
                  accessibilityLabel={t("login-walletconnect-open")}
                  onPress={() => Linking.openURL(uri)}
                >
                  <LoginQr value={uri} size={QR_SIZE} />
                </Pressable>
              ) : (
                <Loader />
              )}
            </View>
          </View>
          {wallets.length > 0 && <OrDivider />}
        </>
      )}
      {wallets.length > 0 && (
        <View style={zero.gap.all[2]}>
          {wallets.map((wallet) => (
            <Button
              key={wallet.info.uuid}
              testID={`login-wallet-${wallet.info.rdns}`}
              variant="secondary"
              leftIcon={
                wallet.info.icon ? (
                  <Image
                    source={{ uri: wallet.info.icon }}
                    style={{ width: 18, height: 18 }}
                  />
                ) : undefined
              }
              onPress={() => connectInjected(wallet)}
              loading={busy === wallet.info.uuid}
              disabled={!!busy}
              style={square}
            >
              {t("login-connect-wallet", { wallet: wallet.info.name })}
            </Button>
          ))}
        </View>
      )}
      {!wallets.length && !walletConnectProjectId && (
        <Text size="sm" color="muted">
          {t("login-no-browser-wallet")}
        </Text>
      )}
      {error ? (
        <Text testID="login-error" size="sm" color={theme.colors.danger}>
          {error}
        </Text>
      ) : null}
      <Button
        testID="login-ethereum-back"
        variant="ghost"
        onPress={onBack}
        style={square}
      >
        {t("back")}
      </Button>
    </View>
  );
}
