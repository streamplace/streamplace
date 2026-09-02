import { $Typed } from "@atproto/api";
import {
  Link,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import { Facet, RichtextSegment, segmentize } from "@streamplace/core";
import { memo, useCallback } from "react";
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

import { useLivestreamStore } from "../../livestream-store";
import { Text } from "../ui/text";
import { BadgeDisplayRow } from "./badge";
import {
  ProfileCardContent,
  UserProfileCard,
  useProfileCardData,
} from "./user-profile-card";

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
          Linking.openURL(`https://bsky.app/profile/${mtnFtr.did || ""}`)
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
  const userCache = useLivestreamStore((state) => state.authors);
  if (!facets?.length) return <Text>{text}</Text>;

  let segs = segmentize(text, facets as Facet[]);

  return segs.map((seg, i) => renderSegment(seg, i, userCache));
};

// Web flows the whole message inline inside a single <Text>, with the badges and
// handle rendered as an inline-block via display: "inline".
// Chat is dense but legible: 14px (size base), handles in medium weight.
const MessageBodyWeb = ({ item }: { item: ChatMessageViewHydrated }) => {
  return (
    <Text size="base" style={[flex.shrink[1], { minWidth: 0 }]}>
      <UserProfileCard uri={item.uri} author={item.author} badges={item.badges}>
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
          <BadgeDisplayRow badges={item.badges} />
          <Text
            size="base"
            weight="medium"
            style={{
              cursor: "pointer",
              color: getRgbColor(item.chatProfile?.color),
            }}
          >
            {formatHandleWithAt(item.author)}
          </Text>
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
  const { theme } = useTheme();
  const data = useProfileCardData(item.author, item.badges);
  return (
    <DropdownMenu
      style={[
        layout.flex.row,
        flex.shrink[1],
        { minWidth: 0, alignItems: "flex-start" },
      ]}
    >
      {!!item.badges?.length && (
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
            <BadgeDisplayRow badges={item.badges} />
          </Pressable>
        </DropdownMenuTrigger>
      )}
      <Text size="base" style={[flex.shrink[1], { minWidth: 0 }]}>
        <DropdownMenuTrigger asChild>
          <Text
            size="base"
            weight="medium"
            style={{ color: getRgbColor(item.chatProfile?.color) }}
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
                  color: getRgbColor(replyTo.chatProfile?.color),
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
