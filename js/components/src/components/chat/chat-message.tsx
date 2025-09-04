import { $Typed } from "@atproto/api";
import {
  Link,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import { memo, useCallback } from "react";
import { Linking, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { RichtextSegment, segmentize } from "../../lib/facet";
import { borders, flex, gap, ml, mr, opacity, pl } from "../../lib/theme/atoms";
import { atoms, colors, layout } from "../ui";

interface Facet {
  index: {
    byteStart: number;
    byteEnd: number;
  };
  features: Array<{
    $type: string;
    uri?: string;
    did?: string;
  }>;
}

import { useLivestreamStore } from "../../livestream-store";
import { Text } from "../ui/text";

const getRgbColor = (color?: { red: number; green: number; blue: number }) =>
  color ? `rgb(${color.red}, ${color.green}, ${color.blue})` : colors.gray[500];

const segmentedObject = (
  obj: RichtextSegment,
  index: number,
  userCache?: { [key: string]: ChatMessageViewHydrated["chatProfile"] },
) => {
  if (obj.features && obj.features.length > 0) {
    let ftr = obj.features[0];
    // afaik there shouldn't be a case where facets overlap, at least currently
    if (ftr.$type === "app.bsky.richtext.facet#link") {
      let linkftr = ftr as $Typed<Link>;
      return (
        <Text
          key={`mention-${index}`}
          style={[{ color: atoms.colors.ios.systemBlue, cursor: "pointer" }]}
          onPress={() => Linking.openURL(linkftr.uri || "")}
        >
          {obj.text}
        </Text>
      );
    } else if (ftr.$type === "app.bsky.richtext.facet#mention") {
      let mtnftr = ftr as $Typed<Mention>;
      const profile = userCache?.[mtnftr.did];
      return (
        <Text
          key={`mention-${index}`}
          style={[
            {
              cursor: "pointer",
              color: getRgbColor(profile?.color),
            },
          ]}
          onPress={() =>
            Linking.openURL(`https://bsky.app/profile/${mtnftr.did || ""}`)
          }
        >
          {obj.text}
        </Text>
      );
    }
  } else {
    return <Text key={`text-${index}`}>{obj.text}</Text>;
  }
};

const RichTextMessage = ({
  text,
  facets,
}: {
  text: string;
  facets: ChatMessageViewHydrated["record"]["facets"];
}) => {
  if (!facets?.length) return <Text>{text}</Text>;

  const userCache = useLivestreamStore((state) => state.authors);

  let segs = segmentize(text, facets as Facet[]);

  return segs.map((seg, i) => segmentedObject(seg, i, userCache));
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
    return (
      <>
        {item.replyTo && showReply && (
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
                  color: getRgbColor((item.replyTo.chatProfile as any).color),
                  fontWeight: "thin",
                }}
              >
                @{(item.replyTo.author as any).handle}
              </Text>{" "}
              <Text
                style={{
                  color: colors.gray[300],
                  fontStyle: "italic",
                }}
              >
                {(item.replyTo.record as any).text}
              </Text>
            </Text>
          </View>
        )}
        <View
          style={[
            gap.all[2],
            layout.flex.row,
            { minWidth: 0, maxWidth: "100%" },
          ]}
        >
          {showTime && (
            <Text
              style={{
                fontVariant: ["tabular-nums"],
                color: colors.gray[400],
              }}
            >
              {formatTime(item.record.createdAt)}
            </Text>
          )}
          <Text
            weight="bold"
            color="default"
            style={[flex.shrink[1], { minWidth: 0, overflow: "hidden" }]}
          >
            <Text
              style={[
                {
                  cursor: "pointer",
                  color: getRgbColor(item.chatProfile?.color),
                },
              ]}
            >
              @{item.author.handle}
            </Text>
            :{" "}
            <RichTextMessage
              text={item.record.text}
              facets={item.record.facets || []}
            />
          </Text>
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
