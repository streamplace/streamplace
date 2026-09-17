import {
  Button,
  Text,
  useDID,
  useFetchAccessStatus,
  useTheme,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import { SiteTitleLockup } from "components/brand/logo";
import { Lock } from "lucide-react-native";
import { useState } from "react";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";

// Shown when the node hasn't answered place.stream.access.getStatus within
// the shell's timeout: rather than guessing (and rendering an app whose every
// request may fail), say so and offer a retry.
export function AccessConnecting({ error }: { error?: string | null }) {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const fetchAccessStatus = useFetchAccessStatus();
  const [retrying, setRetrying] = useState(false);

  const handleRetry = async () => {
    setRetrying(true);
    try {
      await fetchAccessStatus();
    } finally {
      setRetrying(false);
    }
  };

  return (
    <View
      style={[
        zero.flex.values[1],
        zero.layout.flex.align.center,
        zero.layout.flex.justify.center,
        zero.px[6],
        zero.py[12],
        { backgroundColor: theme.colors.background },
      ]}
    >
      <View
        style={{
          maxWidth: 440,
          width: "100%",
          alignItems: "center",
          gap: 16,
        }}
      >
        <SiteTitleLockup size={22} style={{ marginBottom: 8 }} />
        <Text size="lg" weight="semibold" style={{ textAlign: "center" }}>
          {t("access-wall-connecting-title")}
        </Text>
        <Text
          style={{ textAlign: "center", color: theme.colors.textMuted }}
          leading="relaxed"
        >
          {t("access-wall-connecting-body")}
        </Text>
        {error ? (
          <Text
            size="xs"
            style={{ textAlign: "center", color: theme.colors.textMuted }}
          >
            {error}
          </Text>
        ) : null}
        <Button
          variant="primary"
          onPress={handleRetry}
          loading={retrying}
          disabled={retrying}
          style={{ marginTop: 4 }}
        >
          {t("try-again")}
        </Button>
      </View>
    </View>
  );
}

// Full-screen gate shown instead of the navigator when this node's viewer
// role is gated (allowlist/off) and the caller doesn't hold it. Two states:
// logged out -> ask them to sign in; logged in -> tell them they're not on
// the list, with sign-out and a retry (for after an admin grants them).
export default function AccessWall() {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const did = useDID();
  const profile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const logout = useStore((state) => state.logout);
  const fetchAccessStatus = useFetchAccessStatus();
  const [retrying, setRetrying] = useState(false);
  const [signingOut, setSigningOut] = useState(false);

  const loggedIn = !!did;
  const account = profile?.handle ? `@${profile.handle}` : did;

  const handleRetry = async () => {
    setRetrying(true);
    try {
      await fetchAccessStatus();
    } finally {
      setRetrying(false);
    }
  };

  const handleSignOut = async () => {
    setSigningOut(true);
    try {
      await logout();
    } catch (e) {
      console.error("failed to sign out", e);
    } finally {
      setSigningOut(false);
    }
  };

  return (
    <View
      style={[
        zero.flex.values[1],
        zero.layout.flex.align.center,
        zero.layout.flex.justify.center,
        zero.px[6],
        zero.py[12],
        { backgroundColor: theme.colors.background },
      ]}
    >
      <View
        style={{
          maxWidth: 440,
          width: "100%",
          alignItems: "center",
          gap: 16,
        }}
      >
        <SiteTitleLockup size={22} style={{ marginBottom: 8 }} />

        <View
          style={{
            width: 64,
            height: 64,
            borderRadius: 32,
            backgroundColor: theme.colors.muted,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Lock size={30} color={theme.colors.primary} />
        </View>

        <Text size="2xl" weight="semibold" style={{ textAlign: "center" }}>
          {loggedIn
            ? t("access-wall-denied-title")
            : t("access-wall-private-title")}
        </Text>

        {loggedIn && account && (
          <Text
            size="sm"
            style={{ textAlign: "center", color: theme.colors.text2 }}
          >
            {t("access-wall-signed-in-as", { account })}
          </Text>
        )}

        <Text
          style={{ textAlign: "center", color: theme.colors.textMuted }}
          leading="relaxed"
        >
          {loggedIn
            ? t("access-wall-denied-body")
            : t("access-wall-private-body")}
        </Text>

        {loggedIn ? (
          <View style={{ width: "100%", gap: 8, marginTop: 4 }}>
            <Button
              variant="primary"
              onPress={handleRetry}
              loading={retrying}
              disabled={retrying || signingOut}
            >
              {t("try-again")}
            </Button>
            <Button
              variant="secondary"
              onPress={handleSignOut}
              loading={signingOut}
              disabled={retrying || signingOut}
            >
              {t("access-wall-sign-out")}
            </Button>
          </View>
        ) : (
          <Button
            variant="primary"
            onPress={() => openLoginModal()}
            style={{ marginTop: 4 }}
          >
            {t("sign-in")}
          </Button>
        )}
      </View>
    </View>
  );
}
