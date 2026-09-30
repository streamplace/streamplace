import {
  Button,
  Text,
  useNetworkName,
  useTheme,
} from "@streamplace/components";
import { KeyRound } from "lucide-react-native";
import { View } from "react-native";
import { useStore } from "store";

/**
 * Whether the signed-in session is one the node can't attribute: a session
 * inherited from the network's app (brokered) or made from a password
 * (credential). Those can watch and chat, but anything the node itself
 * authorizes — going live, admin screens — needs a node OAuth session.
 */
export function useBearerSession(): boolean {
  const kind = useStore((state) => state.sessionKind);
  return kind === "brokered" || kind === "credential";
}

/**
 * Shown in place of a screen that needs a node OAuth session when the
 * viewer only holds a bearer session. The button opens the login modal on
 * the OAuth flow and returns here afterwards.
 */
export function NodeSessionRequired({
  returnRoute,
  what = "Going live",
}: {
  returnRoute?: { name: string; params?: any };
  what?: string;
}) {
  const { theme } = useTheme();
  const network = useNetworkName();
  const openLoginModal = useStore((state) => state.openLoginModal);
  return (
    <View
      style={{
        flex: 1,
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
        backgroundColor: theme.colors.background,
      }}
    >
      <View
        style={{ maxWidth: 440, width: "100%", alignItems: "center", gap: 16 }}
      >
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
          <KeyRound size={30} color={theme.colors.primary} />
        </View>
        <Text size="2xl" weight="semibold" style={{ textAlign: "center" }}>
          {what} needs a full sign-in
        </Text>
        <Text
          style={{
            textAlign: "center",
            color: theme.colors.text2,
            fontSize: 15,
            lineHeight: 22,
          }}
        >
          You're signed in through {network}, which is enough to watch and chat.{" "}
          {what} is authorized by this node itself, so it needs a session the
          node can verify. Signing in with OAuth takes a moment and brings you
          straight back here.
        </Text>
        <Button
          variant="accent"
          onPress={() => openLoginModal(returnRoute, { oauth: true })}
          style={{ marginTop: 4, alignSelf: "stretch" }}
        >
          Sign in with OAuth
        </Button>
      </View>
    </View>
  );
}
