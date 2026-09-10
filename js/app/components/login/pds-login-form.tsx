import {
  Button,
  Text,
  useBrandingAsset,
  useNetworkName,
  useTheme,
} from "@streamplace/components";
import { LogoMark } from "components/brand/logo";
import { Image } from "expo-image";
import { Lock, Mail } from "lucide-react-native";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  ActivityIndicator,
  LayoutChangeEvent,
  Linking,
  Platform,
  Pressable,
  TextInput,
  View,
} from "react-native";
import { useStore } from "store";
import { BearerSessionData } from "streamplace";

// Sign-in against the node's configured PDS (branding key loginMode=pds):
// email or handle plus password (an app password works too) with the PDS's
// email 2FA, and beside it the PDS's QR / app login (its
// eu.wsocial.quicklogin methods). Either way ends in a credentials session
// this app holds and refreshes itself; the node sees an anonymous viewer.
// The layout follows the network's own sign-in page: QR left, form right
// on wide containers, stacked on narrow ones.

const TWO_COLUMN_MIN = 640;
const QR_POLL_MS = 2000;

interface QuickLoginInit {
  sessionId: string;
  sessionToken: string;
  qrCodeUrl?: string;
  signUrl?: string;
  expiresAt?: string;
}

function usePdsConfig() {
  const pdsUrl = (useBrandingAsset("loginPdsUrl")?.data ?? "")
    .trim()
    .replace(/\/$/, "");
  const quickLogin = useBrandingAsset("quickLogin")?.data === "on";
  const forgotUrl = useBrandingAsset("loginForgotUrl")?.data?.trim();
  const waitlistUrl = useBrandingAsset("loginWaitlistUrl")?.data?.trim();
  const supportEmail = useBrandingAsset("loginSupportEmail")?.data?.trim();
  return { pdsUrl, quickLogin, forgotUrl, waitlistUrl, supportEmail };
}

function toSession(pdsUrl: string, r: any): BearerSessionData | null {
  if (!r || typeof r.did !== "string" || typeof r.accessJwt !== "string") {
    return null;
  }
  return {
    did: r.did,
    handle: typeof r.handle === "string" ? r.handle : r.did,
    service: pdsUrl,
    accessJwt: r.accessJwt,
    refreshJwt: typeof r.refreshJwt === "string" ? r.refreshJwt : undefined,
  };
}

/** The QR / app login: init, show the code, poll until the app signs. */
/**
 * The sign-in deep link encoded in a quick-login QR image URL, e.g.
 * https://auth.example/QR/tagsign:auth.example,ABC?w=400 →
 * "tagsign:auth.example,ABC". Undefined when the URL isn't of that shape.
 */
/**
 * Whether this is a device that would hand a sign-in off to an app on the
 * same device rather than scan a code with a second one: native, or a web
 * browser with a coarse pointer and no hover (phones and tablets; not a
 * narrow desktop window, which still wants the QR).
 */
function useHandoffDevice(): boolean {
  const [touch, setTouch] = useState(() => {
    if (Platform.OS !== "web") return true;
    if (typeof window === "undefined" || !window.matchMedia) return false;
    return window.matchMedia("(hover: none) and (pointer: coarse)").matches;
  });
  useEffect(() => {
    if (Platform.OS !== "web" || typeof window === "undefined") return;
    if (!window.matchMedia) return;
    const mq = window.matchMedia("(hover: none) and (pointer: coarse)");
    const onChange = () => setTouch(mq.matches);
    mq.addEventListener?.("change", onChange);
    return () => mq.removeEventListener?.("change", onChange);
  }, []);
  return touch;
}

/** A readable line for a quick-login failure code from the PDS. */
function describeQuickLoginError(code: string | undefined, network: string) {
  switch (code) {
    case "InvitationRequired":
      return `That identity doesn't have a ${network} account yet, and this sign-in can't create one. Sign up in the ${network} app first, then try again.`;
    case undefined:
    case "":
      return "Login failed.";
    default:
      return code;
  }
}

// Quick-login status methods, in the order to try them.
const STATUS_METHODS = [
  "io.trustanchor.quicklogin.status",
  "eu.wsocial.quicklogin.status",
] as const;

export function signUrlFromQrImage(imageUrl: string): string | undefined {
  try {
    const m = new URL(imageUrl).pathname.match(/\/QR\/(.+)$/);
    return m?.[1] ? decodeURIComponent(m[1]) : undefined;
  } catch {
    return undefined;
  }
}

function QuickLoginPanel({
  pdsUrl,
  compact,
  onSession,
}: {
  pdsUrl: string;
  compact: boolean;
  onSession: (s: BearerSessionData) => void;
}) {
  const { theme } = useTheme();
  const networkName = useNetworkName();
  const [state, setState] = useState<{
    init?: QuickLoginInit;
    error?: string;
    loading: boolean;
  }>({ loading: true });
  const timers = useRef<{ poll?: any; refresh?: any }>({});
  const gen = useRef(0);

  const clearTimers = () => {
    clearInterval(timers.current.poll);
    clearTimeout(timers.current.refresh);
    timers.current = {};
  };

  const start = useCallback(async () => {
    clearTimers();
    const my = ++gen.current;
    setState({ loading: true });
    try {
      const res = await fetch(`${pdsUrl}/xrpc/eu.wsocial.quicklogin.init`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ allowCreate: false }),
      });
      if (!res.ok) throw new Error(`Could not start (${res.status})`);
      const init: QuickLoginInit = await res.json();
      if (my !== gen.current) return;
      // The identity app's deep link is the text inside the QR code, and
      // the QR image URL carries that text as its path: .../QR/<text>. Init
      // doesn't return it separately, so lift it out (the image's own size
      // query string falls away with the path).
      if (!init.signUrl && init.qrCodeUrl) {
        init.signUrl = signUrlFromQrImage(init.qrCodeUrl);
      }
      setState({ init, loading: false });
      if (init.expiresAt) {
        const msLeft = new Date(init.expiresAt).getTime() - Date.now() - 5000;
        if (msLeft > 0) timers.current.refresh = setTimeout(start, msLeft);
      }
      // The status method has two names with one shape and one store
      // behind them: io.trustanchor.quicklogin.status, which every
      // deployment answers, and eu.wsocial.quicklogin.status, which only
      // some do (and a missing method surfaces as a 401, not a 404). Start
      // with the one that is everywhere; fall back if a server lacks it.
      let statusMethod: (typeof STATUS_METHODS)[number] = STATUS_METHODS[0];
      timers.current.poll = setInterval(async () => {
        try {
          const r = await fetch(`${pdsUrl}/xrpc/${statusMethod}`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              sessionId: init.sessionId,
              sessionToken: init.sessionToken,
            }),
          });
          if (!r.ok) {
            if ([400, 401, 404, 501].includes(r.status)) {
              const i = STATUS_METHODS.indexOf(statusMethod);
              statusMethod = STATUS_METHODS[(i + 1) % STATUS_METHODS.length];
            }
            return;
          }
          const data = await r.json();
          if (data?.status === "completed" && data.result) {
            clearTimers();
            const session = toSession(pdsUrl, data.result);
            if (session) onSession(session);
          } else if (data?.status === "failed") {
            clearTimers();
            setState({
              loading: false,
              error: describeQuickLoginError(data.error, networkName),
            });
          }
        } catch {
          // network blip: keep polling
        }
      }, QR_POLL_MS);
    } catch (e: any) {
      if (my !== gen.current) return;
      setState({ loading: false, error: e?.message || "Could not connect." });
    }
  }, [pdsUrl, onSession, networkName]);

  useEffect(() => {
    start();
    return () => {
      gen.current++;
      clearTimers();
    };
  }, [start]);

  const retry = (
    <Pressable onPress={start}>
      <Text style={{ color: theme.colors.primary, fontSize: 13 }}>Retry</Text>
    </Pressable>
  );

  // A phone or tablet with a hand-off link: the app card (a code can't be
  // scanned from the device showing it). Everything else, a narrow desktop
  // window included: the code, centered above the form when narrow.
  const handoff = useHandoffDevice();
  if (
    (handoff && state.init?.signUrl) ||
    (compact && state.error && !state.init)
  ) {
    return (
      <View
        style={{
          alignItems: "center",
          gap: 8,
          padding: 16,
          borderRadius: 12,
          backgroundColor: theme.colors.surface2,
        }}
      >
        <Text weight="semibold" style={{ fontSize: 15 }}>
          {networkName} Identity
        </Text>
        {state.error ? (
          <>
            <Text style={{ color: theme.colors.danger, fontSize: 13 }}>
              {state.error}
            </Text>
            {retry}
          </>
        ) : (
          <Button
            variant="accent"
            // Full width: "min" pins the button to the card's left edge.
            width="full"
            style={{ borderRadius: 999 }}
            onPress={() => Linking.openURL(state.init!.signUrl!)}
          >
            Authenticate
          </Button>
        )}
      </View>
    );
  }

  return (
    <View
      style={{
        alignItems: "center",
        gap: 10,
        width: compact ? "100%" : 220,
      }}
    >
      <View
        style={{
          width: 200,
          height: 200,
          borderRadius: 12,
          overflow: "hidden",
          backgroundColor: theme.colors.surface2,
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        {state.loading ? (
          <ActivityIndicator color={theme.colors.primary} />
        ) : state.error ? (
          <View style={{ alignItems: "center", gap: 6, padding: 12 }}>
            <Text style={{ color: theme.colors.danger, fontSize: 13 }} center>
              {state.error}
            </Text>
            {retry}
          </View>
        ) : state.init?.qrCodeUrl ? (
          <Image
            source={{ uri: state.init.qrCodeUrl }}
            style={{ width: 200, height: 200 }}
            contentFit="contain"
            accessibilityLabel={`Scan to sign in with ${networkName} Identity`}
          />
        ) : null}
      </View>
      <Text style={{ color: theme.colors.text3, fontSize: 12 }} center>
        {state.loading ? "Preparing code…" : "Waiting for scan…"}
      </Text>
    </View>
  );
}

export default function PdsLoginForm({
  onSuccess,
  inModal = false,
  onUseOAuth,
}: {
  onSuccess?: () => void;
  inModal?: boolean;
  /** Switch to the node's OAuth flow (the only session the node can attribute). */
  onUseOAuth?: () => void;
}) {
  const { theme } = useTheme();
  const networkName = useNetworkName();
  const { pdsUrl, quickLogin, forgotUrl, waitlistUrl, supportEmail } =
    usePdsConfig();
  const setCredentialSession = useStore((s) => s.setCredentialSession);
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [needsCode, setNeedsCode] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [width, setWidth] = useState(0);
  const twoColumn = width >= TWO_COLUMN_MIN;

  const finish = useCallback(
    async (session: BearerSessionData) => {
      await setCredentialSession(session);
      onSuccess?.();
    },
    [setCredentialSession, onSuccess],
  );

  const submit = async () => {
    const id = identifier.trim();
    if (busy || !id || !password || !pdsUrl) return;
    setBusy(true);
    setError(null);
    try {
      const body: Record<string, string> = { identifier: id, password };
      if (code.trim()) body.authFactorToken = code.trim().toUpperCase();
      const res = await fetch(
        `${pdsUrl}/xrpc/com.atproto.server.createSession`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        },
      );
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        if (data?.error === "AuthFactorTokenRequired") {
          setNeedsCode(true);
        } else {
          setError(
            data?.message || "Sign in failed. Please check your credentials.",
          );
        }
        return;
      }
      const session = toSession(pdsUrl, data);
      if (!session) {
        setError("Unexpected answer from the sign-in server.");
        return;
      }
      await finish(session);
    } catch {
      setError("Network error. Please check your connection and try again.");
    } finally {
      setBusy(false);
    }
  };

  const canSubmit =
    !busy && !!identifier.trim() && !!password && (!needsCode || !!code.trim());
  const field = (
    icon: React.ReactNode,
    label: string,
    children: React.ReactNode,
  ) => (
    <View style={{ gap: 6 }}>
      <Text style={{ fontSize: 13, color: theme.colors.text2 }}>{label}</Text>
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: 10,
          paddingLeft: 12,
          borderRadius: 10,
          backgroundColor: theme.colors.surface2,
        }}
      >
        {icon}
        <View style={{ flex: 1 }}>{children}</View>
      </View>
    </View>
  );
  const textInputStyle = {
    flex: 1,
    color: theme.colors.text1,
    fontSize: 15,
    paddingVertical: 12,
    paddingRight: 12,
    outlineStyle: "none",
  } as any;

  if (!pdsUrl) {
    return (
      <Text style={{ color: theme.colors.danger }}>
        Sign-in is not configured: set the login PDS in Branding.
      </Text>
    );
  }

  return (
    <View
      onLayout={(e: LayoutChangeEvent) => setWidth(e.nativeEvent.layout.width)}
      style={{ gap: 20, width: "100%" }}
    >
      {!inModal && (
        <View style={{ alignItems: "center", gap: 12 }}>
          <LogoMark size={64} />
          <Text weight="semibold" style={{ fontSize: 28, lineHeight: 34 }}>
            Sign in
          </Text>
          <Text style={{ color: theme.colors.text2, fontSize: 14 }} center>
            {quickLogin && twoColumn
              ? "Scan the code, or use email and password."
              : quickLogin
                ? `Authenticate using ${networkName} Identity or sign in with email and password.`
                : "Sign in with your email and password."}
          </Text>
        </View>
      )}

      <View
        style={{
          flexDirection: twoColumn ? "row" : "column",
          gap: 24,
          alignItems: twoColumn ? "flex-start" : "stretch",
        }}
      >
        {quickLogin && (
          <QuickLoginPanel
            pdsUrl={pdsUrl}
            compact={!twoColumn}
            onSession={finish}
          />
        )}

        <View style={{ flex: 1, gap: 14 }}>
          {field(
            <Mail size={16} color={theme.colors.primary} />,
            "Email",
            <TextInput
              placeholder="Your email"
              value={identifier}
              onChangeText={setIdentifier}
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="email-address"
              textContentType="username"
              style={textInputStyle}
              placeholderTextColor={theme.colors.text3}
              onSubmitEditing={submit}
            />,
          )}
          {field(
            <Lock size={16} color={theme.colors.primary} />,
            "Password",
            <TextInput
              placeholder="Your password"
              value={password}
              onChangeText={setPassword}
              secureTextEntry
              textContentType="password"
              style={textInputStyle}
              placeholderTextColor={theme.colors.text3}
              onSubmitEditing={submit}
            />,
          )}
          {needsCode && (
            <View style={{ gap: 6 }}>
              <Text style={{ fontSize: 13, color: theme.colors.text2 }}>
                2FA confirmation
              </Text>
              <Text style={{ fontSize: 12, color: theme.colors.text3 }}>
                A sign in code has been sent to your email address.
              </Text>
              {field(
                <Lock size={16} color={theme.colors.primary} />,
                "",
                <TextInput
                  placeholder="Confirmation code"
                  value={code}
                  onChangeText={setCode}
                  autoCapitalize="characters"
                  autoCorrect={false}
                  textContentType="oneTimeCode"
                  style={textInputStyle}
                  placeholderTextColor={theme.colors.text3}
                  onSubmitEditing={submit}
                />,
              )}
            </View>
          )}
          {error && (
            <View
              style={{
                padding: 10,
                borderRadius: 8,
                backgroundColor: theme.colors.dangerSoft,
              }}
            >
              <Text style={{ color: theme.colors.danger, fontSize: 13 }}>
                {error}
              </Text>
            </View>
          )}
          <Button
            variant="accent"
            onPress={submit}
            disabled={!canSubmit}
            loading={busy}
            style={{ borderRadius: 10, height: 44 }}
          >
            Sign in
          </Button>
          {forgotUrl ? (
            <Pressable onPress={() => Linking.openURL(forgotUrl)}>
              <Text style={{ color: theme.colors.text2, fontSize: 13 }}>
                Forgot password?
              </Text>
            </Pressable>
          ) : null}
        </View>
      </View>

      {(waitlistUrl || supportEmail) && (
        <View style={{ gap: 6, paddingTop: 8 }}>
          {waitlistUrl ? (
            <Text style={{ color: theme.colors.text3, fontSize: 13 }}>
              If you don't have an account or haven't received an invitation,
              please{" "}
              <Text
                style={{ color: theme.colors.primary, fontSize: 13 }}
                onPress={() => Linking.openURL(waitlistUrl)}
              >
                join the waitlist
              </Text>
              .
            </Text>
          ) : null}
          {supportEmail ? (
            <Text style={{ color: theme.colors.text3, fontSize: 13 }}>
              Having trouble?{" "}
              <Text
                style={{ color: theme.colors.primary, fontSize: 13 }}
                onPress={() => Linking.openURL(`mailto:${supportEmail}`)}
              >
                Contact support
              </Text>
            </Text>
          ) : null}
        </View>
      )}
      {onUseOAuth ? (
        <Pressable onPress={onUseOAuth}>
          <Text style={{ color: theme.colors.text3, fontSize: 13 }}>
            Advanced: sign in with OAuth instead
          </Text>
        </Pressable>
      ) : null}
    </View>
  );
}
