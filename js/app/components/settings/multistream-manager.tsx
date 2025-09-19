import { Text } from "@streamplace/components";
import { ThemeProvider } from "@streamplace/components/src/lib/theme/theme";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import {
  bg,
  borders,
  flex,
  gap,
  h,
  layout,
  mb,
  mt,
  mx,
  p,
  pt,
  r,
  text,
  w,
} from "@streamplace/components/src/ui";
import { Edit3, Trash2 } from "@tamagui/lucide-icons";
import Loading from "components/loading/loading";
import { useEffect, useState } from "react";
import { Alert, Pressable, ScrollView, View } from "react-native";
import {
  PlaceStreamMultistreamDefs,
  PlaceStreamMultistreamTarget,
} from "streamplace";
import { timeAgo } from "utils/timeAgo";

interface MultistreamTargetViewHydrated
  extends PlaceStreamMultistreamDefs.TargetView {
  record: PlaceStreamMultistreamTarget.Record;
}

export default function MultistreamManager() {
  const agent = usePDSAgent();
  const [loading, setLoading] = useState(true);
  const [targets, setTargets] = useState<
    MultistreamTargetViewHydrated[] | null
  >(null);

  const loadMultistreamTargets = async () => {
    if (!agent) return;

    try {
      setLoading(true);
      const targetViews = await agent.place.stream.multistream.listTargets({
        limit: 50,
      });
      setTargets(targetViews.data.targets as MultistreamTargetViewHydrated[]);
    } catch (error) {
      console.error("Failed to load multistream targets:", error);
      Alert.alert(
        "Error",
        "Failed to load multistream targets. Please try again.",
      );
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadMultistreamTargets();
  }, [agent]);

  return (
    <ThemeProvider>
      <View style={[flex.values[1]]}>
        <ScrollView style={[flex.values[1]]}>
          <View style={[{ maxWidth: 800 }, mx.auto]}>
            {/* Header */}
            <View style={[mb[6]]}>
              <Text style={[mb[2], { fontSize: 24, fontWeight: "700" }]}>
                Multistream Targets
              </Text>
              <Text style={[text.gray[400], mb[4], { fontSize: 14 }]}>
                Automatically push your Streamplace livestreams to other
                streaming services like Twitch or YouTube.
              </Text>
            </View>
          </View>
        </ScrollView>

        {/* Content */}
        {loading ? (
          <Loading />
        ) : targets === null ? (
          <View style={[layout.flex.center, mt[8]]}>
            <Text style={[text.gray[600]]}>
              Failed to load multistream targets
            </Text>
          </View>
        ) : targets.length === 0 ? (
          <View style={[layout.flex.center, mt[8]]}>
            <Text style={[text.gray[600], mb[4], { fontSize: 16 }]}>
              No targets yet!
            </Text>
          </View>
        ) : (
          <>
            <View style={[mb[4]]}>
              <Text style={[text.gray[600], { fontSize: 14 }]}>
                {targets.length} target{targets.length !== 1 && "s"}
              </Text>
            </View>
            {targets.map((target) => (
              <MultistreamRow
                key={target.uri}
                target={target}
                onEdit={(uri, record) => {}}
                onDelete={(uri) => {}}
                isDeleting={false}
                // onEdit={handleEdit}
                // onDelete={deleteWebhook}
                // isDeleting={deletingWebhooks.has(webhook.id)}
              />
            ))}
          </>
        )}
      </View>
    </ThemeProvider>
  );
}

export function MultistreamRow({
  target,
  onEdit,
  onDelete,
  isDeleting,
}: {
  target: MultistreamTargetViewHydrated;
  onEdit: (uri: string, record: PlaceStreamMultistreamTarget.Record) => void;
  onDelete: (uri: string) => void;
  isDeleting: boolean;
}) {
  return (
    <View
      style={[
        flex.shrink[1],
        borders.width.thin,
        borders.color.gray[200],
        bg.neutral[800],
        r.xl,
        p[4],
        mb[3],
        layout.flex.column,
        gap.all[3],
        { opacity: isDeleting ? 0.5 : target.record.active ? 1 : 0.7 },
      ]}
    >
      {/* Header */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
          <View
            style={[
              w[3],
              h[3],
              r.full,
              { backgroundColor: target.record.active ? "#22c55e" : "#6b7280" },
            ]}
          />
          <Text style={[{ fontSize: 16, fontWeight: "600" }]}>
            {target.record.name || "Untitled Target"}
          </Text>
        </View>

        <View style={[layout.flex.row, gap.all[2]]}>
          <Pressable
            style={[
              bg.gray[100],
              p[2],
              r.md,
              layout.flex.center,
              { minWidth: 32, minHeight: 32 },
            ]}
            onPress={() => onEdit(target.uri, target.record)}
            disabled={isDeleting}
          >
            <Edit3 size={16} color="#374151" />
          </Pressable>

          <Pressable
            style={[
              bg.red[800],
              p[2],
              r.md,
              layout.flex.center,
              { minWidth: 32, minHeight: 32 },
            ]}
            onPress={() => onDelete(target.uri)}
            disabled={isDeleting}
          >
            <Trash2 size={16} />
          </Pressable>
        </View>
      </View>

      {/* URL */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <Text style={[text.gray[300], { fontSize: 12 }]}>URL:</Text>
        <Text
          style={[text.gray[400], { fontSize: 12, fontFamily: "monospace" }]}
          numberOfLines={1}
        >
          {target.record.url.length > 50
            ? target.record.url.slice(0, 45) +
              "..." +
              target.record.url.slice(target.record.url.length - 5)
            : target.record.url}
        </Text>
      </View>

      {/* Status info */}
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          pt[2],
          borders.top.width.thin,
          borders.top.color.gray[100],
        ]}
      >
        <Text style={[text.gray[400], { fontSize: 11 }]}>
          Created {timeAgo(new Date(target.record.createdAt))}
        </Text>
      </View>
    </View>
  );
}
