import {
  Button,
  Input,
  Loader,
  Text,
  useTheme,
  zero,
} from "@streamplace/components";
import useActorTypeahead from "hooks/useActorTypeahead";
import {
  ArrowRightToLine,
  AtSign,
  CornerDownLeft,
  Info,
} from "lucide-react-native";
import { useEffect, useMemo, useState } from "react";
import { Alert, Image, Linking, Platform, Pressable, View } from "react-native";
import { useStore } from "store";
import { useLogin } from "store/hooks";

interface LoginFormProps {
  onSuccess?: () => void;
}

export default function LoginForm({ onSuccess }: LoginFormProps) {
  const { theme } = useTheme();
  const loginAction = useStore((state) => state.login);
  const openLoginLink = useStore((state) => state.openLoginLink);
  const authStatus = useStore((state) => state.authStatus);
  const loginState = useLogin();
  const [handle, setHandle] = useState("");
  const [imageLoading, setImageLoading] = useState(false);
  const { actors } = useActorTypeahead(handle);

  const filteredActors = useMemo(
    () => actors.filter((actor) => actor.handle.startsWith(handle)),
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

  const submit = () => {
    if (completionText && isMobile) {
      acceptSuggestion();
      return;
    }
    let clean = handle;
    if (handle.startsWith("@")) clean = handle.slice(1);
    loginAction(clean, openLoginLink);
  };

  const acceptSuggestion = () => {
    if (suggestion) {
      setHandle(suggestion.handle);
    }
  };

  const onSignup = () => {
    loginAction("https://bsky.social", openLoginLink);
  };

  const isMobile = Platform.OS === "ios" || Platform.OS === "android";

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
    if (loginState?.error) {
      Alert.alert("Login error", loginState.error);
    }
  }, [loginState?.error]);

  useEffect(() => {
    if (authStatus === "loggedIn" && onSuccess) {
      onSuccess();
    }
  }, [authStatus, onSuccess]);

  return (
    <>
      <View
        style={[
          zero.layout.flex.row,
          { flexWrap: "wrap" },
          zero.gap.all[1],
          zero.mb[4],
        ]}
      >
        <Text style={[{ color: theme.colors.textMuted }]}>
          Sign in using your handle on the AT Protocol
        </Text>
        <Pressable
          onPress={() => {
            const u = new URL(
              "https://atproto.academy/docs/Authentication/why",
            );
            Linking.openURL(u.toString());
          }}
        >
          <Info size={16} style={{ paddingTop: 4 }} color={theme.colors.ring} />
        </Pressable>
        <Text style={[{ color: theme.colors.textMuted }]}>
          (e.g. your Bluesky handle)
        </Text>
      </View>

      <View style={[zero.mb[4], { position: "relative" }]}>
        <Text style={[{ color: "#aaa", marginBottom: 8 }]}>Handle</Text>
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
                    color: "#555",
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
                  color="#555"
                  style={{
                    bottom: 2,
                  }}
                />
              ) : (
                <ArrowRightToLine
                  height={18}
                  color="#555"
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
                <AtSign size={20} color="#eee" />
              </View>
            )}
          </View>
          <Input
            value={handle}
            onChangeText={(text) =>
              setHandle(
                text
                  .toLowerCase()
                  .replace(/[\u202A\u202C\u200E\u200F\u2066-\u2069]/g, "")
                  .trim(),
              )
            }
            placeholder="jcsalterego.bsky.social"
            onKeyPress={onKeyPress}
            onSubmitEditing={submit}
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="url"
            placeholderTextColor="#666"
            containerStyle={{
              paddingLeft: 46,
            }}
          />
        </View>
      </View>

      <View
        style={[
          zero.layout.flex.row,
          { justifyContent: "flex-end", zIndex: -32 },
          zero.gap.all[3],
        ]}
      >
        <Button width="min" onPress={() => onSignup()} variant="ghost">
          <Text style={[{ color: "white" }]}>Sign Up on Bluesky</Text>
        </Button>
        <Button
          onPress={submit}
          disabled={loginState.loading}
          style={[zero.px[6]]}
          width="min"
          loading={loginState.loading}
        >
          <Text style={[{ color: "white" }]}>Log in</Text>
        </Button>
      </View>
    </>
  );
}
