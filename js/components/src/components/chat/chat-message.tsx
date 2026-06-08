import { $Typed } from "@atproto/api";
import {
  Link,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import { Facet, RichtextSegment, segmentize } from "@streamplace/core";
import { memo, useCallback } from "react";
import { Linking, Platform, Pressable, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { borders, flex, gap, ml, mr, opacity, pl } from "../../lib/theme/atoms";
import { formatHandleWithAt } from "../../utils/format-handle";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  atoms,
  colors,
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

const getRgbColor = (color?: { red: number; green: number; blue: number }) =>
  color ? `rgb(${color.red}, ${color.green}, ${color.blue})` : colors.gray[500];

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
    const linkFtr = ftr as $Typed<Link>;
    return (
      <Text
        key={`link-${index}`}
        style={{ color: atoms.colors.ios.systemBlue, cursor: "pointer" }}
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

// Web flows the whole message inline inside a single <Text>, with the badges +
// handle rendered as an inline-block via display: "inline".
const MessageBodyWeb = ({ item }: { item: ChatMessageViewHydrated }) => {
  return (
    <Text style={[flex.shrink[1], { minWidth: 0 }]}>
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
            style={{
              cursor: "pointer",
              color: getRgbColor(item.chatProfile?.color),
            }}
          >
            {formatHandleWithAt(item.author)}
          </Text>
        </View>
      </UserProfileCard>
      <Text color="default">{": "}</Text>
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
              height: 24,
              // iOS centers glyphs higher in the line box than Android
              marginTop: Platform.OS === "ios" ? -2 : 0,
            }}
          >
            <BadgeDisplayRow badges={item.badges} />
          </Pressable>
        </DropdownMenuTrigger>
      )}
      <Text style={[flex.shrink[1], { minWidth: 0 }]}>
        <DropdownMenuTrigger asChild>
          <Text style={{ color: getRgbColor(item.chatProfile?.color) }}>
            {formatHandleWithAt(item.author)}
          </Text>
        </DropdownMenuTrigger>
        <Text color="default">{": "}</Text>
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
              borders.left.width.medium,
              borders.left.color.gray[700],
              ml[4],
              pl[4],
              opacity[80],
            ]}
          >
            <Text
              numberOfLines={1}
              style={[
                flex.shrink[1],
                mr[4],
                { minWidth: 0, overflow: "hidden" },
              ]}
            >
              <Text
                style={{
                  color: getRgbColor(replyTo.chatProfile?.color),
                  fontWeight: "thin",
                }}
              >
                {formatHandleWithAt(replyTo.author)}
              </Text>{" "}
              <Text
                style={{
                  color: colors.gray[300],
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
              style={{
                fontVariant: ["tabular-nums"],
                color: colors.gray[400],
                marginRight: 8,
                marginTop: Platform.OS === "web" ? 1 : 2,
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
