import {
  Button,
  Text,
  useSiteTitle,
  useTheme,
  useTranslation,
  zero,
} from "@streamplace/components";
import { Image } from "expo-image";
import { useEffect, useRef, useState } from "react";
import { Linking, View } from "react-native";
import type { Eip1193Provider } from "./ethereum-session";
import { LoginQr } from "./login-qr";

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

// Sign in with Ethereum's wallet choice (web only): the browser's own
// wallets, and WalletConnect when the node has a project id for it.
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
  const [error, setError] = useState<string | null>(null);
  const pairing = useRef<AbortController | null>(null);
  useEffect(() => () => pairing.current?.abort(), []);
  const square = { borderRadius: theme.borderRadius.none };

  const run = async (
    id: string,
    signIn: (eth: typeof import("./ethereum-sign-in")) => Promise<void>,
  ) => {
    setError(null);
    setBusy(id);
    try {
      await signIn(await import("./ethereum-sign-in"));
    } catch (e: any) {
      if (!isRejection(e)) setError(e?.message ?? String(e));
    } finally {
      setBusy(null);
      setUri(null);
    }
  };

  const walletConnect = () => {
    pairing.current?.abort();
    const signal = (pairing.current = new AbortController()).signal;
    run("walletconnect", (eth) =>
      eth.signInWithWalletConnect({
        projectId: walletConnectProjectId!,
        siteTitle,
        onUri: setUri,
        signal,
      }),
    );
  };

  const back = () => {
    pairing.current?.abort();
    onBack();
  };

  return (
    <View style={zero.gap.all[4]}>
      {uri ? (
        <View style={[zero.layout.flex.alignCenter, zero.gap.all[3]]}>
          <LoginQr value={uri} size={200} />
          <Text size="sm" color="muted">
            {t("login-walletconnect-scan")}
          </Text>
          <Button
            variant="secondary"
            onPress={() => Linking.openURL(uri)}
            style={square}
          >
            {t("login-walletconnect-open")}
          </Button>
        </View>
      ) : (
        <View style={zero.gap.all[2]}>
          {wallets.map(({ info, provider }) => (
            <Button
              key={info.uuid}
              testID={`login-wallet-${info.rdns}`}
              variant="secondary"
              leftIcon={
                info.icon ? (
                  <Image
                    source={{ uri: info.icon }}
                    style={{ width: 18, height: 18 }}
                  />
                ) : undefined
              }
              onPress={() =>
                run(info.uuid, (eth) =>
                  eth.signInWithInjected(provider, siteTitle),
                )
              }
              loading={busy === info.uuid}
              disabled={!!busy}
              style={square}
            >
              {info.name}
            </Button>
          ))}
          {walletConnectProjectId && (
            <Button
              testID="login-walletconnect"
              variant="secondary"
              onPress={walletConnect}
              loading={busy === "walletconnect"}
              disabled={!!busy}
              style={square}
            >
              {t("login-walletconnect")}
            </Button>
          )}
          {!wallets.length && !walletConnectProjectId && (
            <Text size="sm" color="muted">
              {t("login-no-browser-wallet")}
            </Text>
          )}
        </View>
      )}
      {error ? (
        <Text testID="login-error" size="sm" color={theme.colors.danger}>
          {error}
        </Text>
      ) : null}
      <Button
        testID="login-ethereum-back"
        variant="ghost"
        onPress={back}
        style={square}
      >
        {t("back")}
      </Button>
    </View>
  );
}
