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

export interface WalletConnectPairing {
  uri: string | null;
  error: string | null;
  /** Drop this pairing and start a fresh one (the user left the panel). */
  restart: () => void;
}

// A WalletConnect pairing started in the background while the login form is
// up, so the QR code is ready when the user opens the Ethereum panel. It
// re-pairs when a proposal expires (after five minutes) or the wallet turns
// it down. Null without a project id.
export function useWalletConnectPairing(
  projectId: string | undefined,
): WalletConnectPairing | null {
  const siteTitle = useSiteTitle();
  const [attempt, setAttempt] = useState(0);
  const [uri, setUri] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    if (!projectId) return;
    // aborted when this pairing is dropped, so a wallet that approves it
    // afterwards isn't asked for a signature
    const pairing = new AbortController();
    setUri(null);
    setError(null);
    import("./ethereum-sign-in")
      .then((eth) =>
        eth.signInWithWalletConnect({
          projectId,
          siteTitle,
          // an aborted attempt's link would pair with nothing that signs
          onUri: (uri) => {
            if (!pairing.signal.aborted) setUri(uri);
          },
          signal: pairing.signal,
        }),
      )
      .catch((e) => {
        if (pairing.signal.aborted) return;
        if (isRejection(e) || /expired/i.test(message(e))) {
          setAttempt((n) => n + 1);
        } else {
          setError(message(e));
        }
      });
    return () => pairing.abort();
  }, [projectId, siteTitle, attempt]);
  if (!projectId) return null;
  return { uri, error, restart: () => setAttempt((n) => n + 1) };
}

// Sign in with Ethereum (web only): the WalletConnect QR code when the node
// has a project id for it, then the browser's own wallets.
export function EthereumWallets({
  walletConnect,
  onBack,
}: {
  walletConnect: WalletConnectPairing | null;
  onBack: () => void;
}) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const siteTitle = useSiteTitle();
  const wallets = useInjectedWallets(t("login-browser-wallet"));
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // aborted when the user leaves the list (Back unmounts it), so a wallet
  // still connecting then doesn't go on to ask for a signature
  const [left] = useState(() => new AbortController());
  useEffect(() => () => left.abort(), [left]);
  const square = { borderRadius: theme.borderRadius.none };
  const uri = walletConnect?.uri;

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
      {walletConnect && (
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
              {walletConnect.error ? (
                <Text
                  size="sm"
                  color={theme.colors.danger}
                  style={{ textAlign: "center" }}
                >
                  {walletConnect.error}
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
      {!wallets.length && !walletConnect && (
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
