import {
  Button,
  Checkbox,
  Input,
  Loader,
  Text,
  Tooltip,
  useLoginOptions,
  useLoginPlaceholder,
  useNetworkName,
  useTheme,
  useTranslation,
  useUrl,
  zero,
} from "@streamplace/components";
import { formatHandle, useActorTypeahead } from "@streamplace/core";
import { Image } from "expo-image";
import {
  ArrowRightToLine,
  AtSign,
  CornerDownLeft,
  Info,
} from "lucide-react-native";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import { Linking, Platform, Pressable, View } from "react-native";
import { useStore } from "store";
import { useLogin } from "store/hooks";
import { EthereumWallets, useWalletConnectPairing } from "./ethereum-wallets";
import { LoginQr } from "./login-qr";
import { OrDivider } from "./or-divider";
import { AppleIcon, EthereumIcon, GoogleIcon } from "./provider-icons";
import { useQrLogin } from "./use-qr-login";

// An email address means the entryway's own email + password sign-in.
const EMAIL = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;
const QR_SIZE = 160;
const DEFAULT_SIGN_UP = "https://streamplace.com";
const isMobile = Platform.OS === "ios" || Platform.OS === "android";

interface LoginFormProps {
  onSuccess?: () => void;
}

export default function LoginForm({ onSuccess }: LoginFormProps) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const network = useNetworkName();
  const loginPlaceholder = useLoginPlaceholder();
  const { entryway, methods, walletConnectProjectId } = useLoginOptions();
  const url = useUrl();
  const loginAction = useStore((state) => state.login);
  const openLoginLink = useStore((state) => state.openLoginLink);
  const authStatus = useStore((state) => state.authStatus);
  const loginState = useLogin();
  const [handle, setHandle] = useState("");
  const [imageLoading, setImageLoading] = useState(false);
  const [blueskyPermissions, setBlueskyPermissions] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [ethereumOpen, setEthereumOpen] = useState(false);
  const isEmail = EMAIL.test(handle);
  const { actors } = useActorTypeahead(isEmail ? "" : handle);

  // an unverifiable account formats as its DID, so it can't complete a handle
  const filteredActors = useMemo(
    () => actors.filter((actor) => formatHandle(actor).startsWith(handle)),
    [actors, handle],
  );

  const suggestion = useMemo(
    () =>
      filteredActors.length > 0 &&
      handle.length >= 3 &&
      filteredActors[0].handle.startsWith(handle)
        ? filteredActors[0]
        : null,
    [filteredActors],
  );

  const completionText = useMemo(
    () =>
      suggestion && suggestion.handle
        ? suggestion.handle.slice(handle.length)
        : null,
    [suggestion, handle],
  );

  const avatarUri = useMemo(() => suggestion?.avatar, [suggestion?.avatar]);

  const providers = (["apple", "google"] as const).filter((m) =>
    methods.includes(m),
  );
  // browser wallets and WalletConnect's web flow; the app gets its own later
  const showEthereum = Platform.OS === "web" && methods.includes("ethereum");
  // the QR is for the app to scan, so only a browser shows it
  const showQr = Platform.OS === "web" && methods.includes("qr");
  const qrUrl = useQrLogin(blueskyPermissions);
  const walletConnect = useWalletConnectPairing(
    showEthereum ? walletConnectProjectId : undefined,
  );
  const signUpUrl = entryway ?? DEFAULT_SIGN_UP;
  const square = { borderRadius: theme.borderRadius.none };
  const shownError = error ?? loginState.error;

  const submit = () => {
    if (completionText && isMobile) {
      acceptSuggestion();
      return;
    }
    setError(null);
    const clean = handle.startsWith("@") ? handle.slice(1) : handle;
    if (EMAIL.test(clean)) {
      if (!entryway) {
        setError(t("login-email-needs-entryway"));
        return;
      }
      loginAction(entryway, openLoginLink, {
        blueskyPermissions,
        screen: "sign-in",
      });
      return;
    }
    loginAction(clean, openLoginLink, { blueskyPermissions });
  };

  const signInWithProvider = (provider: "apple" | "google") => {
    setError(null);
    loginAction(entryway!, openLoginLink, { blueskyPermissions, provider });
  };

  const acceptSuggestion = () => {
    if (suggestion) {
      setHandle(suggestion.handle);
    }
  };

  const onKeyPress = (e: any) => {
    if (e.nativeEvent.key === "Enter") {
      submit();
    } else if (e.nativeEvent.key === "Tab" && completionText) {
      e.preventDefault();
      acceptSuggestion();
    } else if (e.nativeEvent.key === "ArrowRight" && completionText) {
      const input = e.target;
      if (input.selectionStart === handle.length) {
        e.preventDefault();
        acceptSuggestion();
      }
    } else if (e.nativeEvent.key === " " && completionText) {
      e.preventDefault();
      acceptSuggestion();
    }
  };

  useEffect(() => {
    if (authStatus === "loggedIn" && onSuccess) {
      onSuccess();
    }
  }, [authStatus, onSuccess]);

  if (ethereumOpen) {
    return (
      <EthereumWallets
        walletConnect={walletConnect}
        onBack={() => {
          setEthereumOpen(false);
          // a pairing whose QR has been on screen may be in a wallet's hands
          walletConnect?.restart();
        }}
      />
    );
  }

  // logo-only, side by side; the label is for screen readers
  const providerButton = (
    testID: string,
    label: string,
    icon: ReactNode,
    onPress: () => void,
  ) => (
    <Button
      testID={testID}
      accessibilityLabel={label}
      variant="secondary"
      onPress={onPress}
      disabled={loginState.loading}
      style={[square, { flex: 1, width: "auto" }]}
    >
      {icon}
    </Button>
  );

  return (
    <View style={zero.gap.all[4]}>
      {showQr && qrUrl !== null && (
        <>
          <View
            testID="login-qr"
            style={[zero.layout.flex.alignCenter, zero.gap.all[3]]}
          >
            <View
              style={[
                zero.layout.flex.alignCenter,
                zero.layout.flex.justifyCenter,
                { width: QR_SIZE, height: QR_SIZE },
              ]}
            >
              {qrUrl ? <LoginQr value={qrUrl} size={QR_SIZE} /> : <Loader />}
            </View>
            <Text size="sm" color="muted">
              {t("login-scan-qr")}
            </Text>
          </View>
          <OrDivider />
        </>
      )}
      {(providers.length > 0 || showEthereum) && (
        <>
          <View style={[zero.layout.flex.row, zero.gap.all[2]]}>
            {providers.includes("apple") &&
              providerButton(
                "login-apple",
                t("login-continue-with-apple"),
                <AppleIcon size={20} color={theme.colors.text1} />,
                () => signInWithProvider("apple"),
              )}
            {providers.includes("google") &&
              providerButton(
                "login-google",
                t("login-continue-with-google"),
                <GoogleIcon size={20} />,
                () => signInWithProvider("google"),
              )}
            {showEthereum &&
              providerButton(
                "login-ethereum",
                t("login-continue-with-ethereum"),
                <EthereumIcon size={20} color={theme.colors.text1} />,
                () => setEthereumOpen(true),
              )}
          </View>
          <OrDivider />
        </>
      )}

      <View style={zero.gap.all[2]}>
        <View
          style={[
            zero.layout.flex.row,
            zero.layout.flex.alignCenter,
            zero.gap.all[1],
          ]}
        >
          <Text size="sm" color="muted">
            {entryway ? t("login-handle-or-email") : t("login-handle")}
          </Text>
          <Pressable
            accessibilityRole="link"
            accessibilityLabel={t("sign-in-description", { network })}
            onPress={() => Linking.openURL("https://atmosphereaccount.com/")}
          >
            <Info size={14} color={theme.colors.text3} />
          </Pressable>
        </View>
        <View style={{ position: "relative" }}>
          {completionText && suggestion?.handle !== handle ? (
            <View
              style={[
                {
                  position: "absolute",
                  left: 13 + 28 + 6,
                  top: isMobile ? 15.25 : 12,
                  zIndex: 1000000,
                  pointerEvents: "none",
                },
                zero.layout.flex.row,
                zero.layout.flex.align.center,
                zero.gap.all[1],
              ]}
            >
              <Text
                style={[
                  {
                    color: theme.colors.text4,
                    pointerEvents: "none",
                    zIndex: 1000000,
                    fontSize: 16,
                  },
                ]}
              >
                <Text
                  style={{
                    opacity: 0.2,
                    fontSize: 16,
                  }}
                >
                  {handle}
                </Text>
                {completionText}
              </Text>
              {isMobile ? (
                <CornerDownLeft
                  height={18}
                  color={theme.colors.text4}
                  style={{
                    bottom: 2,
                  }}
                />
              ) : (
                <ArrowRightToLine
                  height={18}
                  color={theme.colors.text4}
                  style={{
                    paddingBottom: 1,
                  }}
                />
              )}
            </View>
          ) : (
            <></>
          )}
          <View
            style={[
              zero.layout.position.absolute,
              zero.layout.flex.row,
              { zIndex: 32, top: 8, left: 10 },
            ]}
          >
            {avatarUri ? (
              <View
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: 900,
                  justifyContent: "center",
                  alignItems: "center",
                }}
              >
                {imageLoading && (
                  <View
                    style={{
                      position: "absolute",
                      zIndex: 1,
                    }}
                  >
                    <Loader />
                  </View>
                )}
                <Image
                  key={avatarUri}
                  source={{ uri: avatarUri }}
                  style={{
                    width: 32,
                    height: 32,
                    borderRadius: 999,
                    opacity: suggestion?.handle === handle ? 1 : 0.7,
                  }}
                  onLayout={() => setImageLoading(true)}
                  onLoad={() => setImageLoading(false)}
                  onError={() => setImageLoading(false)}
                />
              </View>
            ) : (
              <View
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: 900,
                  justifyContent: "center",
                  alignItems: "center",
                }}
              >
                <AtSign size={20} color={theme.colors.text2} />
              </View>
            )}
          </View>
          <Input
            testID="login-handle"
            value={handle}
            onChangeText={(text) =>
              setHandle(
                text
                  .toLowerCase()
                  .replace(/[\u202A\u202C\u200E\u200F\u2066-\u2069]/g, "")
                  .trim(),
              )
            }
            placeholder={loginPlaceholder ?? "jcsalterego.bsky.social"}
            onKeyPress={onKeyPress}
            onSubmitEditing={submit}
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="email-address"
            placeholderTextColor={theme.colors.textMuted}
            containerStyle={[square, { paddingLeft: 46 }]}
          />
        </View>
        {shownError ? (
          <Text testID="login-error" size="sm" color={theme.colors.danger}>
            {shownError}
          </Text>
        ) : null}
      </View>

      <View
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.gap.all[3],
        ]}
      >
        {/* preview of the Bluesky LIVE ring the user gets while streaming */}
        <View style={{ alignItems: "center", width: 56 }}>
          <View
            style={{
              width: 48,
              height: 48,
              borderRadius: 999,
              borderWidth: 2.5,
              borderColor: blueskyPermissions
                ? theme.colors.live
                : theme.colors.border,
              padding: 2,
              justifyContent: "center",
              alignItems: "center",
            }}
          >
            {avatarUri ? (
              <Image
                source={{ uri: avatarUri }}
                style={{ width: "100%", height: "100%", borderRadius: 999 }}
              />
            ) : (
              <View
                style={{
                  width: "100%",
                  height: "100%",
                  borderRadius: 999,
                  justifyContent: "center",
                  alignItems: "center",
                  backgroundColor: theme.colors.muted,
                }}
              >
                <AtSign size={22} color={theme.colors.text2} />
              </View>
            )}
          </View>
          <View
            style={{
              marginTop: -8,
              backgroundColor: theme.colors.live,
              borderRadius: theme.borderRadius.sm,
              paddingHorizontal: 4,
              paddingVertical: 1,
              opacity: blueskyPermissions ? 1 : 0,
            }}
          >
            <Text
              style={{
                color: theme.colors.liveForeground,
                fontSize: 9,
                fontWeight: "700",
                letterSpacing: 0.5,
              }}
            >
              LIVE
            </Text>
          </View>
        </View>
        <Tooltip
          content={t("login-show-live-on-bluesky-description", { network })}
          position="top"
          style={{ flex: 1 }}
        >
          <Checkbox
            checked={blueskyPermissions}
            onCheckedChange={setBlueskyPermissions}
            label={t("login-show-live-on-bluesky", { network })}
          />
        </Tooltip>
      </View>

      <Button
        testID="login-submit"
        onPress={submit}
        disabled={loginState.loading}
        loading={loginState.loading}
        style={square}
      >
        {t("continue")}
      </Button>

      <View
        style={[
          zero.layout.flex.row,
          zero.layout.flex.justifyCenter,
          zero.gap.all[1],
          { flexWrap: "wrap" },
        ]}
      >
        <Text size="sm" color="muted">
          {t("login-no-account")}
        </Text>
        <Pressable
          testID="login-sign-up"
          accessibilityRole="link"
          onPress={() => Linking.openURL(signUpUrl)}
        >
          <Text size="sm" color={theme.colors.primary}>
            {t("login-sign-up-on", { host: new URL(signUpUrl).host })}
          </Text>
        </Pressable>
      </View>
    </View>
  );
}
