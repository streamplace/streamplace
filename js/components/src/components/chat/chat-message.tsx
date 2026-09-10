import { $Typed } from "@atproto/api";
import {
  Link,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import { Facet, RichtextSegment, segmentize } from "@streamplace/core";
import { memo, useCallback, useMemo } from "react";
import { Linking, Platform, Pressable, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { flex, gap, ml, mr, opacity, pl } from "../../lib/theme/atoms";
import { tabularNums, textAlphas } from "../../lib/theme/tokens";
import { formatHandleWithAt } from "../../utils/format-handle";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  layout,
  useTheme,
} from "../ui";

import { useAvatars } from "../../hooks/useAvatars";
import { useLivestreamStore } from "../../livestream-store";
import {
  useBrandingAsset,
  useNetworkProfileUrl,
} from "../../streamplace-store";
import { Avatar } from "../ui/avatar";
import { Text } from "../ui/text";
import { Badge, BadgeDisplayRow } from "./badge";
import {
  ProfileCardContent,
  UserProfileCard,
  useProfileCardData,
} from "./user-profile-card";
import { VerifiedBadge } from "./verified-badge";

// Deterministic per-user color, muted so a busy chat doesn't turn into a
// rainbow, and clamped for contrast against the dark chat surface: colors
// that are too dark get mixed toward white until they clear a minimum
// luminance. Same input always yields the same output.
const MIN_LUMA = 110; // 0-255 relative luminance floor
const DESAT = 0.45; // how far to pull each color toward its own gray
const clampChannel = (v: number) => Math.max(0, Math.min(255, Math.round(v)));
const getRgbColor = (color?: {
  red: number;
  green: number;
  blue: number;
}): string => {
  if (!color) return textAlphas.dark[2];
  let { red, green, blue } = color;
  const luma = 0.2126 * red + 0.7152 * green + 0.0722 * blue;
  red += (luma - red) * DESAT;
  green += (luma - green) * DESAT;
  blue += (luma - blue) * DESAT;
  if (luma < MIN_LUMA) {
    // mix toward white just enough to reach the floor
    const t = (MIN_LUMA - luma) / (255 - luma || 1);
    red = red + (255 - red) * t;
    green = green + (255 - green) * t;
    blue = blue + (255 - blue) * t;
  }
  return `rgb(${clampChannel(red)}, ${clampChannel(green)}, ${clampChannel(blue)})`; // token-ok: dynamic user color / soft shadow
};

const LinkSegment = ({
  seg,
  index,
}: {
  seg: RichtextSegment;
  index: number;
}) => {
  const { theme } = useTheme();
  const linkFtr = seg.features?.[0] as $Typed<Link>;
  return (
    <Text
      key={`link-${index}`}
      style={{ color: theme.colors.info, cursor: "pointer" }}
      // @ts-ignore href renders as <a> on web
      href={Platform.OS === "web" ? linkFtr.uri : undefined}
      accessibilityRole="link"
      onPress={(e) => {
        if (Platform.OS === "web") {
          e.preventDefault();
          window.open(linkFtr.uri, "_blank");
        } else {
          Linking.openURL(linkFtr.uri || "");
        }
      }}
    >
      {seg.text}
    </Text>
  );
};

const renderSegment = (
  seg: RichtextSegment,
  index: number,
  userCache?: { [key: string]: ChatMessageViewHydrated["chatProfile"] },
  profileUrl?: (author: { handle?: string; did?: string }) => string,
) => {
  const ftr = seg.features?.[0];

  if (!ftr) {
    return <Text key={`text-${index}`}>{seg.text}</Text>;
  }

  if (ftr.$type === "app.bsky.richtext.facet#link") {
    return <LinkSegment key={`link-${index}`} seg={seg} index={index} />;
  }

  if (ftr.$type === "app.bsky.richtext.facet#mention") {
    const mtnFtr = ftr as $Typed<Mention>;
    const profile = userCache?.[mtnFtr.did];
    return (
      <Text
        key={`mention-${index}`}
        style={{ color: getRgbColor(profile?.color), cursor: "pointer" }}
        onPress={() =>
          Linking.openURL(
            profileUrl
              ? profileUrl({ did: mtnFtr.did })
              : `https://bsky.app/profile/${mtnFtr.did || ""}`,
          )
        }
      >
        {seg.text}
      </Text>
    );
  }
  return <Text key={`unknown-facet-${index}`}>{seg.text}</Text>;
};

export const RichTextMessage = ({
  text,
  facets,
}: {
  text: string;
  facets: ChatMessageViewHydrated["record"]["facets"];
}) => {
  const profileUrl = useNetworkProfileUrl();
  const userCache = useLivestreamStore((state) => state.authors);
  if (!facets?.length) return <Text>{text}</Text>;

  let segs = segmentize(text, facets as Facet[]);

  return segs.map((seg, i) => renderSegment(seg, i, userCache, profileUrl));
};

// Web flows the whole message inline inside a single <Text>, with the badges and
// handle rendered as an inline-block via display: "inline".
// Chat is dense but legible: 14px (size base), handles in medium weight.
// Branding key chatNameColors=off: names in the default text color instead
// of each user's chosen chat color.
function useNameColor(): (
  color?: Parameters<typeof getRgbColor>[0],
) => string | undefined {
  const off = useBrandingAsset("chatNameColors")?.data === "off";
  return off ? () => undefined : getRgbColor;
}

// Branding key chatBadges: "custom" drops the node's built-in marks
// (streamer, moderator, bot), "none" drops every badge.
const BUILT_IN_BADGES = new Set([
  "place.stream.badge.defs#streamer",
  "place.stream.badge.defs#mod",
  "place.stream.badge.defs#bot",
]);
function useVisibleBadges(
  badges: ChatMessageViewHydrated["badges"],
): ChatMessageViewHydrated["badges"] {
  const mode = useBrandingAsset("chatBadges")?.data;
  return useMemo(() => {
    if (!badges || mode === "all" || !mode) return badges;
    if (mode === "none") return [];
    return badges.filter((b) => !BUILT_IN_BADGES.has(b.badgeType));
  }, [badges, mode]);
}

const MessageBodyWeb = ({ item }: { item: ChatMessageViewHydrated }) => {
  const nameColor = useNameColor();
  const badges = useVisibleBadges(item.badges);
  const dids = useMemo(() => [item.author.did], [item.author.did]);
  const profile = useAvatars(dids)[item.author.did];
  return (
    <Text size="base" style={[flex.shrink[1], { minWidth: 0 }]}>
      <UserProfileCard uri={item.uri} author={item.author} badges={badges}>
        <View
          style={
            {
              display: "inline",
              alignItems: "center",
              justifyContent: "flex-end",
              flexDirection: "row",
              marginBottom: -6,
            } as any
          }
        >
          <BadgeDisplayRow badges={badges} />
          <Text
            size="base"
            weight="medium"
            style={{
              cursor: "pointer",
              color: nameColor(item.chatProfile?.color),
            }}
          >
            {formatHandleWithAt(item.author)}
          </Text>
          <VerifiedBadge author={item.author} profile={profile} size={14} />
        </View>
      </UserProfileCard>
      <Text size="base" color="default">
        {": "}
      </Text>
      <RichTextMessage
        text={item.record.text}
        facets={item.record.facets || []}
      />
    </Text>
  );
};

// Native can't reliably nest views or images inside <Text>, so the badges sit in
// a flex row beside the message instead of inline. Tapping the badges or the
// handle opens the same profile bottom sheet via two triggers on one menu.
const MessageBodyNative = ({ item }: { item: ChatMessageViewHydrated }) => {
  const nameColor = useNameColor();
  const badges = useVisibleBadges(item.badges);
  const dids = useMemo(() => [item.author.did], [item.author.did]);
  const profile = useAvatars(dids)[item.author.did];
  const { theme } = useTheme();
  const data = useProfileCardData(item.author, badges);
  return (
    <DropdownMenu
      style={[
        layout.flex.row,
        flex.shrink[1],
        { minWidth: 0, alignItems: "flex-start" },
      ]}
    >
      {!!badges?.length && (
        <DropdownMenuTrigger asChild>
          <Pressable
            style={{
              flexDirection: "row",
              alignItems: "center",
              // match the base text line height so badges center on the first line
              height: 20,
              // iOS centers glyphs higher in the line box than Android
              marginTop: Platform.OS === "ios" ? 1 : 0,
            }}
          >
            <BadgeDisplayRow badges={badges} />
            <VerifiedBadge author={item.author} profile={profile} size={14} />
          </Pressable>
        </DropdownMenuTrigger>
      )}
      <Text size="base" style={[flex.shrink[1], { minWidth: 0 }]}>
        <DropdownMenuTrigger asChild>
          <Text
            size="base"
            weight="medium"
            style={{ color: nameColor(item.chatProfile?.color) }}
          >
            {formatHandleWithAt(item.author)}
          </Text>
        </DropdownMenuTrigger>
        <Text size="base" color="default">
          {": "}
        </Text>
        <RichTextMessage
          text={item.record.text}
          facets={item.record.facets || []}
        />
      </Text>
      <DropdownMenuContent style={{ minWidth: 280, maxWidth: 320 }}>
        <ProfileCardContent data={data} theme={theme} />
      </DropdownMenuContent>
    </DropdownMenu>
  );
};

// Short relative age for the avatar layout: "now", "3m", "2h", "5d".
function relativeAge(dateString: string): string {
  const diff = Date.now() - new Date(dateString).getTime();
  if (!Number.isFinite(diff) || diff < 45_000) return "now";
  const m = Math.round(diff / 60_000);
  if (m < 60) return `${m}m`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h`;
  return `${Math.round(h / 24)}d`;
}

// The avatar layout (branding chatLayout=avatar): a post-style row with the
// author's avatar, display name, handle and age on one line and the text
// beneath, the way a social timeline shows a reply. Badges sit after the
// name; the profile card opens from the name as it does in compact rows.
const AvatarMessageBody = ({ item }: { item: ChatMessageViewHydrated }) => {
  const { theme } = useTheme();
  const badges = useVisibleBadges(item.badges);
  const author = item.author;
  // Hydrated messages carry handle and display name but not the avatar;
  // the profile cache batches getProfiles across visible rows.
  const dids = useMemo(() => [author.did], [author.did]);
  const profile = useAvatars(dids)[author.did];
  const avatar = profile?.avatar || author.avatar;
  const displayName = (author.displayName || profile?.displayName)?.trim();
  const handle = formatHandleWithAt(author);
  const name = displayName || handle;
  const nameColor = useNameColor()(item.chatProfile?.color);
  const meta = { fontSize: 15, lineHeight: 23, color: theme.colors.text2 };
  // The name line is a flex row rather than nested <Text> so the badges (which
  // are views) sit after the name on every platform, a long handle truncates
  // instead of pushing the age off the edge, and native never nests views in
  // text.
  return (
    <View
      style={[
        layout.flex.row,
        { gap: 12, minWidth: 0, maxWidth: "100%", paddingVertical: 4 },
      ]}
    >
      <Avatar src={avatar} name={displayName || author.handle} size={42} />
      <View style={[flex.shrink[1], { flex: 1, minWidth: 0 }]}>
        <View
          style={[
            layout.flex.row,
            { alignItems: "center", minWidth: 0, maxWidth: "100%" },
          ]}
        >
          <View style={[flex.shrink[1], { minWidth: 0 }]}>
            <UserProfileCard
              uri={item.uri}
              author={item.author}
              badges={badges}
            >
              <Text
                weight="semibold"
                numberOfLines={1}
                style={[
                  flex.shrink[1],
                  {
                    fontSize: 15,
                    lineHeight: 23,
                    color: nameColor,
                    minWidth: 0,
                  },
                ]}
              >
                {name}
              </Text>
            </UserProfileCard>
          </View>
          <VerifiedBadge author={author} profile={profile} />
          {!!badges?.length && (
            <View
              style={[
                layout.flex.row,
                { alignItems: "center", marginLeft: 6, flexShrink: 0 },
              ]}
            >
              {badges.map((badge, index) => (
                <Badge
                  key={index}
                  badgeType={badge.badgeType}
                  imageUrl={badge.imageUrl}
                />
              ))}
            </View>
          )}
          {displayName ? (
            // The handle gives way first: flex shrink is weighted by the
            // factor, so at 1000x the name loses well under a pixel while
            // the handle collapses, and only truncates once it is gone.
            <Text
              numberOfLines={1}
              style={{ ...meta, marginLeft: 8, minWidth: 0, flexShrink: 1000 }}
            >
              {handle}
            </Text>
          ) : null}
          <Text style={{ ...meta, marginLeft: 8, flexShrink: 0 }}>
            {"· " + relativeAge(item.record.createdAt)}
          </Text>
        </View>
        <Text style={{ fontSize: 15, lineHeight: 23 }}>
          <RichTextMessage
            text={item.record.text}
            facets={item.record.facets || []}
          />
        </Text>
      </View>
    </View>
  );
};

export const RenderChatMessage = memo(
  function RenderChatMessage({
    item,
    showReply = true,
    showTime = true,
  }: {
    item: ChatMessageViewHydrated;
    userCache?: Map<string, ChatMessageViewHydrated["chatProfile"]>;
    showReply?: boolean;
    showTime?: boolean;
  }) {
    const { theme } = useTheme();
    const avatarLayout = useBrandingAsset("chatLayout")?.data === "avatar";
    const nameColor = useNameColor();
    const formatTime = useCallback((dateString: string) => {
      return new Date(dateString).toLocaleString(undefined, {
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      });
    }, []);
    const replyTo = (item.replyTo as ChatMessageViewHydrated) || null;
    return (
      <>
        {replyTo && showReply && (
          <View
            style={[
              gap.all[2],
              layout.flex.row,
              { minWidth: 0, maxWidth: "100%" },
              {
                borderLeftWidth: 2,
                borderLeftColor: theme.colors.borderStrong,
              },
              ml[4],
              pl[4],
              opacity[80],
            ]}
          >
            <Text
              size="xs"
              numberOfLines={1}
              style={[
                flex.shrink[1],
                mr[4],
                { minWidth: 0, overflow: "hidden" },
              ]}
            >
              <Text
                size="xs"
                weight="medium"
                style={{
                  color: nameColor(replyTo.chatProfile?.color),
                }}
              >
                {formatHandleWithAt(replyTo.author)}
              </Text>{" "}
              <Text
                size="xs"
                style={{
                  color: theme.colors.text3,
                  fontStyle: "italic",
                }}
              >
                {replyTo.record.text}
              </Text>
            </Text>
          </View>
        )}
        {avatarLayout ? (
          <AvatarMessageBody item={item} />
        ) : (
          <View style={[layout.flex.row, { minWidth: 0, maxWidth: "100%" }]}>
            {showTime && (
              <Text
                size="xs"
                style={{
                  ...tabularNums,
                  color: theme.colors.text3,
                  marginRight: 8,
                  marginTop: Platform.OS === "web" ? 2 : 3,
                }}
              >
                {formatTime(item.record.createdAt)}
              </Text>
            )}
            {Platform.OS === "web" ? (
              <MessageBodyWeb item={item} />
            ) : (
              <MessageBodyNative item={item} />
            )}
          </View>
        )}
      </>
    );
  },
  (prevProps, nextProps) => {
    return (
      prevProps.item.author.handle === nextProps.item.author.handle &&
      prevProps.item.record.text === nextProps.item.record.text &&
      prevProps.item.uri === nextProps.item.uri
    );
  },
);
