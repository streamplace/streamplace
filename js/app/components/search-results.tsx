import {
  Avatar,
  Text,
  useStreamplaceStore,
  useTheme,
} from "@streamplace/components";
import {
  borderRadius,
  spacing,
  touchTargets,
} from "@streamplace/components/src/lib/theme/tokens";
import AQLink from "components/aqlink";
import useAvatars from "hooks/useAvatars";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { View } from "react-native";
import { place } from "streamplace";

// An md avatar, its live ring and row padding fit without changing row height
// when live status arrives. Larger accessibility text can still grow the row.
export const SEARCH_RESULT_HEIGHT = touchTargets.large;

export default function SearchResults({
  actors,
  variant,
  activeIndex = -1,
  onSelect,
}: {
  actors: place.stream.live.searchActorsTypeahead.Actor[];
  variant: "page" | "dropdown";
  activeIndex?: number;
  onSelect?: () => void;
}) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  // Search results carry only DIDs and handles; avatars come from the shared
  // profile cache, fetched in one batch for the whole list.
  const dids = useMemo(() => actors.map((actor) => actor.did), [actors]);
  const profiles = useAvatars(dids);
  const liveUsers = useStreamplaceStore((state) => state.liveUsers);
  const liveAuthors = useMemo(
    () =>
      new Map(liveUsers?.map((stream) => [stream.author.did, stream.author])),
    [liveUsers],
  );
  const dropdown = variant === "dropdown";
  const prefix = dropdown ? "header-search" : "search";

  return (
    <View style={dropdown ? undefined : { gap: spacing[1] }}>
      {actors.map((actor, index) => {
        const testID = `${prefix}-result-${actor.did}`;
        const liveAuthor = liveAuthors.get(actor.did);
        const live = Boolean(liveAuthor);
        return (
          <AQLink
            key={actor.did}
            testID={testID}
            nativeID={testID}
            role={dropdown ? "option" : undefined}
            aria-selected={dropdown ? index === activeIndex : undefined}
            to={{
              screen: "Stream",
              params: { user: actor.handle || actor.did },
            }}
            onPress={onSelect}
            style={{
              flexDirection: "row",
              alignItems: "center",
              gap: spacing[3],
              padding: spacing[2],
              borderRadius: borderRadius.md,
              minHeight: dropdown ? SEARCH_RESULT_HEIGHT : touchTargets.minimum,
              ...(index === activeIndex && {
                backgroundColor: theme.colors.surfaceHover,
              }),
            }}
          >
            <Avatar
              testID={`${prefix}-avatar-${actor.did}`}
              name={actor.handle}
              src={profiles[actor.did]?.avatar ?? liveAuthor?.avatar}
              size={dropdown ? "md" : "lg"}
              live={live}
              accessible
              accessibilityRole="image"
              accessibilityLabel={
                live ? `${actor.handle}, ${t("live-badge")}` : actor.handle
              }
              style={live ? undefined : { margin: spacing[1] }}
            />
            <Text numberOfLines={1} style={{ flex: 1 }}>
              {`@${actor.handle}`}
            </Text>
          </AQLink>
        );
      })}
    </View>
  );
}
