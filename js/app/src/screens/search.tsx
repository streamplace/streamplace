import { Avatar, Button, Input, Text, useTheme } from "@streamplace/components";
import {
  borderRadius,
  breakpoints,
  spacing,
  touchTargets,
} from "@streamplace/components/src/lib/theme/tokens";
import { usePossiblyUnauthedPDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import AQLink from "components/aqlink";
import { EmptyState, EmptyStateTile } from "components/empty-state";
import { AlertCircle, Search } from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ActivityIndicator, Platform, ScrollView, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { place } from "streamplace";

const RESULT_LIMIT = 20;

type SearchState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error" }
  | {
      status: "done";
      query: string;
      actors: place.stream.live.searchActorsTypeahead.Actor[];
    };

// The server matches the start of a handle case-insensitively; a leading "@"
// is how people write handles, not part of one.
function normalizeQuery(text: string): string {
  return text.trim().replace(/^@/, "");
}

export default function SearchScreen() {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const agent = usePossiblyUnauthedPDSAgent();
  const safeAreaInsets = useSafeAreaInsets();
  const [text, setText] = useState("");
  const [state, setState] = useState<SearchState>({ status: "idle" });
  // Bumped on edits, searches, node changes, and unmount so only the latest
  // response is applied.
  const requestId = useRef(0);

  useEffect(() => {
    setState({ status: "idle" });
    return () => {
      requestId.current++;
    };
  }, [agent]);

  const onChangeText = (value: string) => {
    setText(value);
    requestId.current++;
    setState({ status: "idle" });
  };

  const submit = async () => {
    const q = normalizeQuery(text);
    if (!q) return;
    const id = ++requestId.current;
    if (!agent) {
      setState({ status: "error" });
      return;
    }
    setState({ status: "loading" });
    try {
      const res = await agent.client.call(
        place.stream.live.searchActorsTypeahead,
        { q, limit: RESULT_LIMIT },
      );
      if (id === requestId.current) {
        setState({ status: "done", query: q, actors: res.actors });
      }
    } catch (error) {
      console.error("Failed to search users:", error);
      if (id === requestId.current) setState({ status: "error" });
    }
  };

  return (
    <ScrollView
      keyboardShouldPersistTaps="handled"
      contentContainerStyle={{
        flexGrow: 1,
        gap: spacing[4],
        width: "100%",
        maxWidth: breakpoints.sm,
        alignSelf: "center",
        paddingHorizontal: spacing[4],
        // Same insets as the Videos tab root: clear the transparent iOS
        // header and the native tab bar.
        paddingTop:
          Platform.OS === "ios" ? spacing[16] + spacing[8] : spacing[4],
        paddingBottom:
          (Platform.OS !== "web" ? spacing[16] : 0) +
          safeAreaInsets.bottom +
          spacing[4],
      }}
    >
      <View
        style={{
          flexDirection: "row",
          alignItems: "flex-end",
          gap: spacing[2],
        }}
      >
        <View style={{ flex: 1 }}>
          <Input
            testID="search-input"
            label={t("search-users-label")}
            accessibilityLabel={t("search-users-label")}
            placeholder={t("search-users-placeholder")}
            value={text}
            onChangeText={onChangeText}
            onSubmitEditing={submit}
            returnKeyType="search"
            autoCapitalize="none"
            autoCorrect={false}
          />
        </View>
        <Button
          testID="search-submit"
          width="min"
          style={{ alignSelf: "flex-end", minHeight: touchTargets.minimum }}
          leftIcon={<Search size={spacing[4]} />}
          disabled={!normalizeQuery(text)}
          onPress={submit}
        >
          {t("search")}
        </Button>
      </View>

      {state.status === "idle" && (
        <EmptyState
          testID="search-hint"
          illustration={<EmptyStateTile icon={Search} />}
          title={t("search-users-hint")}
        />
      )}

      {state.status === "loading" && (
        <View
          testID="search-loading"
          style={{ flex: 1, alignItems: "center", justifyContent: "center" }}
        >
          <ActivityIndicator
            size="large"
            color={theme.colors.text3}
            accessibilityLabel={t("loading")}
          />
        </View>
      )}

      {state.status === "error" && (
        <EmptyState
          testID="search-error"
          illustration={
            <EmptyStateTile icon={AlertCircle} tone={theme.colors.danger} />
          }
          title={t("search-failed")}
          action={
            <Button width="min" variant="secondary" onPress={submit}>
              {t("try-again")}
            </Button>
          }
        />
      )}

      {state.status === "done" && state.actors.length === 0 && (
        <EmptyState
          testID="search-no-results"
          illustration={<EmptyStateTile icon={Search} />}
          title={t("no-results-for", { query: state.query })}
        />
      )}

      {state.status === "done" && state.actors.length > 0 && (
        <View style={{ gap: spacing[1] }}>
          {state.actors.map((actor) => (
            <AQLink
              key={actor.did}
              testID={`search-result-${actor.did}`}
              to={{
                screen: "Stream",
                params: { user: actor.handle || actor.did },
              }}
              style={{
                flexDirection: "row",
                alignItems: "center",
                gap: spacing[3],
                minHeight: touchTargets.minimum,
                padding: spacing[2],
                borderRadius: borderRadius.md,
              }}
            >
              <Avatar name={actor.handle} size="lg" aria-hidden />
              <Text numberOfLines={1} style={{ flex: 1 }}>
                {`@${actor.handle}`}
              </Text>
            </AQLink>
          ))}
        </View>
      )}
    </ScrollView>
  );
}
