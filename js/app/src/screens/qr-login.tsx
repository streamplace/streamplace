import { useNavigation, useRoute } from "@react-navigation/native";
import {
  Avatar,
  Button,
  Loader,
  Text,
  usePDSAgent,
  usePossiblyUnauthedPDSAgent,
  useTheme,
  useTranslation,
  zero,
} from "@streamplace/components";
import { formatHandle } from "@streamplace/core";
import { STORE_URLS } from "constants/store-urls";
import usePlatform from "hooks/usePlatform";
import { useEffect, useState } from "react";
import { Platform, ScrollView, View } from "react-native";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";
import { place } from "streamplace";
import * as UAParser from "ua-parser-js";
import { timeAgo } from "utils/timeAgo";

type QrLogin = place.stream.login.defs.QrLogin;

// "Chrome on macOS", from the requesting browser's User-Agent
function describeBrowser(userAgent: string) {
  const { browser, os } = UAParser.UAParser(userAgent);
  return [browser.name, os.name].filter(Boolean).join(" on ") || userAgent;
}

// Approve a browser's QR login (login/qr/:id, what its QR code links to):
// the browser is signed in as you, sharing this device's session. Opened by
// the app from the scanned link, or in a browser where you're signed in.
export default function QrLoginScreen() {
  const route = useRoute();
  const navigation = useNavigation();
  const id = (route.params as { id?: string } | undefined)?.id ?? "";
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const { isWebIOS, isWebAndroid } = usePlatform();
  const authStatus = useStore((state) => state.authStatus);
  const openLoginModal = useStore((state) => state.openLoginModal);
  const profile = useUserProfile();
  const anonAgent = usePossiblyUnauthedPDSAgent();
  const agent = usePDSAgent();
  const [login, setLogin] = useState<QrLogin | null | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Signed out: the app (and a desktop browser) signs in first; a phone's
  // browser has no session to share, so it gets the app.
  useEffect(() => {
    if (authStatus !== "loggedOut") return;
    if (Platform.OS === "web" && (isWebIOS || isWebAndroid)) {
      window.location.href = isWebIOS ? STORE_URLS.ios : STORE_URLS.android;
      return;
    }
    openLoginModal({ name: route.name, params: route.params });
  }, [authStatus, isWebIOS, isWebAndroid, openLoginModal, route]);

  useEffect(() => {
    if (authStatus !== "loggedIn" || !anonAgent || !id) return;
    let cancelled = false;
    anonAgent.client
      .call(place.stream.login.getQRLogin, { id })
      .then((l) => !cancelled && setLogin(l))
      .catch(() => !cancelled && setLogin(null));
    return () => {
      cancelled = true;
    };
  }, [authStatus, anonAgent, id]);

  const complete = async (approve: boolean) => {
    if (!agent) return;
    setBusy(true);
    setError(null);
    try {
      setLogin(
        await agent.client.call(place.stream.login.completeQRLogin, {
          id,
          approve,
        }),
      );
    } catch (e: any) {
      setError(e?.message ?? String(e));
    } finally {
      setBusy(false);
    }
  };

  const done = () =>
    (navigation as any).navigate("HomeTab", { screen: "HomeMain" });
  const square = { borderRadius: theme.borderRadius.none };

  let body;
  if (authStatus !== "loggedIn" || login === undefined) {
    body = <Loader />;
  } else if (login?.status === "pending") {
    const rows: [string, string][] = [
      [t("qr-login-device"), describeBrowser(login.userAgent)],
      [t("qr-login-ip"), login.ip],
      [t("qr-login-requested"), timeAgo(new Date(login.createdAt))],
    ];
    body = (
      <View style={zero.gap.all[6]}>
        <Text size="2xl" weight="semibold">
          {t("qr-login-title")}
        </Text>
        {profile && (
          <View
            style={[
              zero.layout.flex.row,
              zero.layout.flex.alignCenter,
              zero.gap.all[3],
            ]}
          >
            <Avatar src={profile.avatar} name={profile.handle} size="md" />
            <View>
              {profile.displayName ? (
                <Text weight="semibold">{profile.displayName}</Text>
              ) : null}
              <Text size="sm" color="muted">
                @{formatHandle(profile)}
              </Text>
            </View>
          </View>
        )}
        <View
          style={[
            zero.gap.all[3],
            zero.p[4],
            {
              borderWidth: 1,
              borderColor: theme.colors.border,
              backgroundColor: theme.colors.surface2,
            },
          ]}
        >
          {rows.map(([label, value]) => (
            <View key={label} style={[zero.layout.flex.row, zero.gap.all[3]]}>
              <Text size="sm" color="muted" style={{ width: 96 }}>
                {label}
              </Text>
              <Text size="sm" style={{ flex: 1 }}>
                {value}
              </Text>
            </View>
          ))}
        </View>
        <Text size="sm" color="muted">
          {t("qr-login-warning")}
        </Text>
        {error ? (
          <Text size="sm" color={theme.colors.danger}>
            {error}
          </Text>
        ) : null}
        <View style={zero.gap.all[2]}>
          <Button
            testID="qr-login-approve"
            onPress={() => complete(true)}
            loading={busy}
            disabled={busy}
            style={square}
          >
            {t("qr-login-approve")}
          </Button>
          <Button
            testID="qr-login-deny"
            variant="secondary"
            onPress={() => complete(false)}
            disabled={busy}
            style={square}
          >
            {t("qr-login-deny")}
          </Button>
        </View>
      </View>
    );
  } else {
    const message =
      login?.status === "approved"
        ? t("qr-login-approved")
        : login?.status === "denied"
          ? t("qr-login-denied")
          : t("qr-login-gone");
    body = (
      <View style={zero.gap.all[6]}>
        <Text testID="qr-login-result" size="xl" weight="semibold">
          {message}
        </Text>
        <Button testID="qr-login-done" onPress={done} style={square}>
          {t("qr-login-done")}
        </Button>
      </View>
    );
  }

  return (
    <ScrollView
      style={{ backgroundColor: theme.colors.background }}
      contentContainerStyle={[
        zero.layout.flex.alignCenter,
        zero.layout.flex.justifyCenter,
        zero.p[6],
        { flexGrow: 1 },
      ]}
    >
      <View style={{ width: "100%", maxWidth: 400 }}>{body}</View>
    </ScrollView>
  );
}
