import { Avatar, Button, Input, Text, useTheme } from "@streamplace/components";
import {
  borderRadius,
  spacing,
  touchTargets,
} from "@streamplace/components/src/lib/theme/tokens";
import { usePossiblyUnauthedPDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import AQLink from "components/aqlink";
import { EmptyState, EmptyStateTile } from "components/empty-state";
import { AlertCircle, Search } from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  ActivityIndicator,
  Keyboard,
  NativeSyntheticEvent,
  Platform,
  ScrollView,
  TextInputKeyPressEventData,
  useWindowDimensions,
  View,
} from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { place } from "streamplace";

const RESULT_LIMIT = 20;
// The dropdown searches while typing: at most one request per window, sent
// at its start and with the latest text at its end.
const THROTTLE_MS = 300;

type Actor = place.stream.live.searchActorsTypeahead.Actor;

type SearchState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error" }
  | { status: "done"; query: string; actors: Actor[] };

type UserSearchProps =
  | { variant: "page" }
  | {
      variant: "dropdown";
      open: boolean;
      onOpenChange: (open: boolean) => void;
    };

// The server matches the start of a handle case-insensitively; a leading "@"
// is how people write handles, not part of one.
function normalizeQuery(text: string): string {
  return text.trim().replace(/^@/, "");
}

function useActorSearch() {
  const agent = usePossiblyUnauthedPDSAgent();
  const [state, setState] = useState<SearchState>({ status: "idle" });
  // Bumped on edits, searches, cancels, node changes, and unmount so only the
  // latest response is applied.
  const requestId = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const lastSearchAt = useRef(0);

  useEffect(() => {
    setState({ status: "idle" });
    return () => {
      clearTimeout(timer.current);
      requestId.current++;
    };
  }, [agent]);

  const search = async (q: string) => {
    clearTimeout(timer.current);
    lastSearchAt.current = Date.now();
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

  const throttledSearch = (q: string) => {
    clearTimeout(timer.current);
    requestId.current++;
    setState({ status: "loading" });
    const wait = lastSearchAt.current + THROTTLE_MS - Date.now();
    if (wait <= 0) search(q);
    else timer.current = setTimeout(() => search(q), wait);
  };

  const cancel = () => {
    clearTimeout(timer.current);
    requestId.current++;
    setState({ status: "idle" });
  };

  return { state, search, throttledSearch, cancel };
}

function ResultLink({
  actor,
  testID,
  avatarSize,
  onPress,
}: {
  actor: Actor;
  testID: string;
  avatarSize: "md" | "lg";
  onPress?: () => void;
}) {
  return (
    <AQLink
      testID={testID}
      to={{ screen: "Stream", params: { user: actor.handle || actor.did } }}
      onPress={onPress}
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: spacing[3],
        minHeight: touchTargets.minimum,
        padding: spacing[2],
        borderRadius: borderRadius.md,
      }}
    >
      <Avatar name={actor.handle} size={avatarSize} aria-hidden />
      <Text numberOfLines={1} style={{ flex: 1 }}>
        {`@${actor.handle}`}
      </Text>
    </AQLink>
  );
}

export default function UserSearch(props: UserSearchProps) {
  return props.variant === "page" ? (
    <PageSearch />
  ) : (
    <DropdownSearch open={props.open} onOpenChange={props.onOpenChange} />
  );
}

function PageSearch() {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const { state, search, cancel } = useActorSearch();
  const [text, setText] = useState("");

  const onChangeText = (value: string) => {
    setText(value);
    cancel();
  };

  const submit = () => {
    const q = normalizeQuery(text);
    if (!q) return;
    if (Platform.OS !== "web") Keyboard.dismiss();
    search(q);
  };

  return (
    <>
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
            accessibilityLabel={t("search")}
            placeholder={t("search")}
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
          title={t("search")}
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
            <ResultLink
              key={actor.did}
              testID={`search-result-${actor.did}`}
              actor={actor}
              avatarSize="lg"
            />
          ))}
        </View>
      )}
    </>
  );
}

function DropdownSearch({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const { height } = useWindowDimensions();
  const { state, throttledSearch, cancel } = useActorSearch();
  const [text, setText] = useState("");

  // Opening (including refocusing retained text) starts a search. Edits while
  // open use the same throttle; empty queries never capture outside presses.
  useEffect(() => {
    if (open) throttledSearch(normalizeQuery(text));
    else cancel();
  }, [open]);

  const onChangeText = (value: string) => {
    setText(value);
    const q = normalizeQuery(value);
    if (q && open) throttledSearch(q);
    else cancel();
    onOpenChange(Boolean(q));
  };

  const onFocus = () => {
    if (normalizeQuery(text)) onOpenChange(true);
  };

  const dismiss = () => {
    onOpenChange(false);
    if (Platform.OS !== "web") Keyboard.dismiss();
  };

  const onKeyPress = (e: NativeSyntheticEvent<TextInputKeyPressEventData>) => {
    if (e.nativeEvent.key === "Escape") dismiss();
  };

  return (
    <View style={{ width: "100%" }}>
      <Input
        testID="header-search-input"
        accessibilityLabel={t("search")}
        placeholder={t("search")}
        value={text}
        onChangeText={onChangeText}
        onFocus={onFocus}
        onPressIn={onFocus}
        onKeyPress={onKeyPress}
        returnKeyType="search"
        autoCapitalize="none"
        autoCorrect={false}
        containerStyle={{ minHeight: touchTargets.minimum }}
      />
      {open && state.status !== "idle" && (
        <Animated.View
          testID="header-search-dropdown"
          entering={FadeInDown.duration(theme.motion.fast).withInitialValues({
            opacity: 0,
            transform: [{ translateY: -spacing[1] }],
          })}
          style={{
            position: "absolute",
            top: "100%",
            left: 0,
            right: 0,
            marginTop: spacing[1],
            backgroundColor: theme.colors.surface2,
            borderWidth: 1,
            borderColor: theme.colors.borderSubtle,
            borderRadius: borderRadius.md,
            overflow: "hidden",
            ...theme.shadows.sm,
          }}
        >
          <ScrollView
            style={{ maxHeight: height / 2 }}
            contentContainerStyle={{ padding: spacing[1] }}
            keyboardShouldPersistTaps="handled"
          >
            {state.status === "loading" && (
              <View
                testID="header-search-loading"
                style={{ padding: spacing[2] }}
              >
                <ActivityIndicator
                  color={theme.colors.text3}
                  accessibilityLabel={t("loading")}
                />
              </View>
            )}

            {state.status === "error" && (
              <Text
                testID="header-search-error"
                color="muted"
                style={{ padding: spacing[2] }}
              >
                {t("search-failed")}
              </Text>
            )}

            {state.status === "done" && state.actors.length === 0 && (
              <Text
                testID="header-search-no-results"
                color="muted"
                style={{ padding: spacing[2] }}
              >
                {t("no-results-for", { query: state.query })}
              </Text>
            )}

            {state.status === "done" &&
              state.actors.map((actor) => (
                <ResultLink
                  key={actor.did}
                  testID={`header-search-result-${actor.did}`}
                  actor={actor}
                  avatarSize="md"
                  onPress={dismiss}
                />
              ))}
          </ScrollView>
        </Animated.View>
      )}
    </View>
  );
}
