import { Button, View, Text } from "tamagui";
import OAuthClient from "atproto/oauth";
import { useEffect, useState } from "react";
import { useAuthContext } from "hooks/useAuthContext";
import { useIsFocused } from "@react-navigation/native";

export default function Login() {
  const [error, setError] = useState<string | null>("");
  const [user, setUser] = useState<string | null>(null);
  const [isInitializing, setIsInitializing] = useState(true);
  const auth = useAuthContext();
  const isFocused = useIsFocused();
  useEffect(() => {
    if (!auth) {
      setUser(null);
      setIsInitializing(false);
      return;
    }
    (async () => {
      setIsInitializing(true);
      const res = await auth.pdsAgent.getProfile({
        actor: auth.pdsAgent.sessionManager.did!,
      });
      setUser(res.data.handle);
      setIsInitializing(false);
    })();
  }, [auth, isFocused]);
  if (user) {
    return (
      <View f={1} jc="center" ai="center">
        <Text>Logged in as @{user}</Text>
        <Button onPress={() => auth!.signOut()}>Log out</Button>
      </View>
    );
  }
  if (isInitializing || auth?.isInitializing) {
    return (
      <View f={1} jc="center" ai="center">
        <Text>Initializing...</Text>
      </View>
    );
  }
  return (
    <View f={1} jc="center" ai="center">
      <Text>{error}</Text>
      <Button
        onPress={async () => {
          try {
            const url = await OAuthClient.authorize("https://bsky.social");
            document.location.href = url.toString();
          } catch (e) {
            setError(e.message);
          }
        }}
      >
        Log in with Bluesky
      </Button>
    </View>
  );
}
// http://127.0.0.1:38081/?iss=https%3A%2F%2Fbsky.social&state=_GOY1291lhsCoXRLemN32g&code=cod-e1039a2f58fee83243ccb8b5114d21097feb78d6e230f97d02c5a27727cf9f99
