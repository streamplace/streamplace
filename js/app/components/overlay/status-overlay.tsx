import {
  borderAlphas,
  borderRadius,
  spacing,
  statusColors,
  surfaces,
  tabularNums,
  textAlphas,
  typeScale,
  useLivestreamStore,
  withAlpha,
} from "@streamplace/components";
import { StyleSheet, Text, View } from "react-native";
import type { OverlayProps } from "./registry";

/**
 * The starter overlay: what this widget is talking to, and whether the
 * upstream websocket is up. It is the overlay an operator points OBS at while
 * they are setting a scene up, and it exercises both halves of the shell —
 * the websocket (connected / live / viewers) and the version manifest (the
 * node build it is showing).
 */
export default function StatusOverlay({ user, version }: OverlayProps) {
  const websocketConnected = useLivestreamStore((s) => s.websocketConnected);
  const profile = useLivestreamStore((s) => s.profile);
  const livestream = useLivestreamStore((s) => s.livestream);
  const viewers = useLivestreamStore((s) => s.viewers);

  const handle = profile?.handle ?? user;

  return (
    <View style={styles.root} testID="overlay-status">
      <View style={styles.card}>
        <View style={styles.header}>
          <View
            style={[
              styles.dot,
              {
                backgroundColor: websocketConnected
                  ? statusColors.dark.success
                  : statusColors.dark.warning,
              },
            ]}
          />
          <Text style={styles.headerText} testID="overlay-upstream">
            {websocketConnected ? "Connected" : "Connecting"}
          </Text>
        </View>

        <Row label="Streamer" value={handle} testID="overlay-streamer" />
        <Row
          label="Node"
          value={version ?? "unknown"}
          testID="overlay-version"
        />
        <Row
          label="Live"
          value={livestream ? (livestream.record.title ?? "yes") : "offline"}
        />
        {viewers !== null && (
          <Row label="Viewers" value={String(viewers)} tabular />
        )}
      </View>
    </View>
  );
}

function Row({
  label,
  value,
  testID,
  tabular = false,
}: {
  label: string;
  value: string;
  testID?: string;
  tabular?: boolean;
}) {
  return (
    <View style={styles.row}>
      <Text style={styles.rowLabel}>{label}</Text>
      <Text style={[styles.rowValue, tabular && tabularNums]} testID={testID}>
        {value}
      </Text>
    </View>
  );
}

// Composited over the operator's scene rather than a themed app surface, so
// the card is always the dark scheme (see the design skill's OBS-root case).
const styles = StyleSheet.create({
  root: {
    flex: 1,
    alignItems: "flex-start",
    justifyContent: "flex-start",
    padding: spacing[4],
  },
  card: {
    gap: spacing[2],
    minWidth: 220,
    padding: spacing[4],
    borderRadius: borderRadius.md,
    borderWidth: 1,
    borderColor: borderAlphas.dark.default,
    backgroundColor: withAlpha(surfaces.dark[0], 0.72),
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing[2],
  },
  dot: {
    width: spacing[2],
    height: spacing[2],
    borderRadius: borderRadius.full,
  },
  headerText: {
    ...typeScale.xs,
    fontWeight: "600",
    letterSpacing: 1,
    textTransform: "uppercase",
    color: textAlphas.dark[2],
  },
  row: {
    flexDirection: "row",
    justifyContent: "space-between",
    gap: spacing[4],
  },
  rowLabel: {
    ...typeScale.sm,
    color: textAlphas.dark[2],
  },
  rowValue: {
    ...typeScale.sm,
    color: textAlphas.dark[1],
    flexShrink: 1,
  },
});
