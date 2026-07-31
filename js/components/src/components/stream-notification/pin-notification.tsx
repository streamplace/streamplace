import { EyeOff, Pin, X } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Linking, Pressable, View } from "react-native";
import { PinnedRecordViewHydrated, place } from "streamplace";
import {
  Text,
  useCanModerate,
  useLivestreamStore,
  useTheme,
  zero,
} from "../../";
import { RichtextSegment, segmentize } from "../../lib/facet";
import * as tokens from "../../lib/theme/tokens";
import { formatHandleWithAt } from "../../utils/format-handle";

// static link accent — this overlay can render outside a ThemeProvider (OBS)
const LINK_ACCENT = tokens.colors.primary[400]; // token-ok

const getRgbColor = (color?: place.stream.chat.profile.Color) =>
  color ? `rgb(${color.red}, ${color.green}, ${color.blue})` : undefined; // token-ok: dynamic user color / soft shadow

function renderSegment(segment: RichtextSegment, index: number) {
  if (segment.features && segment.features.length > 0) {
    const ftr = segment.features[0];
    if (ftr.$type === "app.bsky.richtext.facet#link") {
      return (
        <Text
          key={index}
          style={[{ color: LINK_ACCENT }]}
          onPress={() => Linking.openURL((ftr as any).uri || "")}
        >
          {segment.text}
        </Text>
      );
    }
    if (ftr.$type === "app.bsky.richtext.facet#mention") {
      return (
        <Text key={index} style={[{ color: LINK_ACCENT }]}>
          {segment.text}
        </Text>
      );
    }
  }
  return <Text key={index}>{segment.text}</Text>;
}

export function PinnedCommentNotification({
  pinnedComment,
  onDismiss,
  onUnpin,
}: {
  pinnedComment: PinnedRecordViewHydrated;
  onDismiss: () => void;
  onUnpin: () => void;
}) {
  const z = useTheme();
  const message = pinnedComment.message;
  const pinnedByColor =
    (pinnedComment.pinnedBy as any)?.color || tokens.textAlphas.dark[2];
  const record = pinnedComment.record;

  const currentStreamer = useLivestreamStore((state) => state.profile?.did);

  console.log("checking if we can mod", currentStreamer);

  const canActuallyPin = useCanModerate(currentStreamer)?.canPin;

  const messageRecord = message?.record as any;

  const [expiresAt] = useState<Date | null>(
    record.expiresAt ? new Date(record.expiresAt) : null,
  );

  useEffect(() => {
    if (!expiresAt) return;
    const remaining = expiresAt.getTime() - Date.now();
    if (remaining <= 0) {
      onDismiss();
      return;
    }
    const timeout = setTimeout(onDismiss, remaining);
    return () => clearTimeout(timeout);
  }, [expiresAt, onDismiss]);

  const authorName = message ? formatHandleWithAt(message.author) : "unknown";
  const authorColor = getRgbColor((message as any)?.chatProfile?.color);

  const segments = messageRecord
    ? segmentize(messageRecord.text, messageRecord.facets)
    : [];

  return (
    <View style={[zero.bg.neutral[900], zero.r.lg, { overflow: "hidden" }]}>
      <View
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.px[3],
          zero.py[2],
          { gap: 8 },
        ]}
      >
        <View style={{ transform: [{ rotate: "-25deg" }] }}>
          <Pin
            size={24}
            color={authorColor || z.theme.colors.primary}
            fill={authorColor || z.theme.colors.primary}
          />
        </View>
        <View
          style={[zero.layout.flex.column, zero.flex.values[1], { gap: 4 }]}
        >
          <View style={[zero.layout.flex.row, { gap: 4, flexWrap: "wrap" }]}>
            <Text
              style={[
                {
                  fontWeight: "600",
                  color: authorColor || tokens.textAlphas.dark[1],
                },
              ]}
            >
              {authorName}
            </Text>
            {segments.map((seg, i) => renderSegment(seg, i))}
          </View>
        </View>
        <View style={[zero.layout.flex.row, { gap: 4 }]}>
          {canActuallyPin && (
            <Pressable onPress={onUnpin} style={{ padding: 4 }}>
              <X size={16} color={tokens.textAlphas.dark[2]} />
            </Pressable>
          )}
          <Pressable onPress={onDismiss} style={{ padding: 4 }}>
            <EyeOff size={16} color={tokens.textAlphas.dark[2]} />
          </Pressable>
        </View>
      </View>
    </View>
  );
}
