import {
  borderAlphas,
  borderRadius,
  spacing,
  surfaces,
  textAlphas,
  typeScale,
  withAlpha,
} from "@streamplace/components";
import { StyleSheet, Text, View } from "react-native";
import { OVERLAYS } from "./registry";

/**
 * What an operator sees in OBS when the overlay URL is wrong. An overlay is
 * addressed only by its URL, so there is no UI to complain: this card has to
 * be legible over the scene they are building, and it lists the names that
 * would work.
 */
export function OverlayDiagnostic({
  title,
  detail,
}: {
  title: string;
  detail: string;
}) {
  return (
    <View style={styles.root} testID="overlay-diagnostic">
      <View style={styles.card}>
        <Text style={styles.title}>{title}</Text>
        <Text style={styles.detail}>{detail}</Text>
        <Text style={styles.names}>
          Available overlays: {Object.keys(OVERLAYS).sort().join(", ")}
        </Text>
      </View>
    </View>
  );
}

// Composited over the operator's scene rather than a themed app surface, so
// the card is always the dark scheme (see the design skill's OBS-root case).
const styles = StyleSheet.create({
  root: {
    // Absolute fill: the diagnostic renders outside the widget wrapper, and
    // has to own the window whatever the navigator's flex layout is.
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    alignItems: "center",
    justifyContent: "center",
    padding: spacing[4],
  },
  card: {
    gap: spacing[2],
    maxWidth: 420,
    padding: spacing[4],
    borderRadius: borderRadius.md,
    borderWidth: 1,
    borderColor: borderAlphas.dark.default,
    backgroundColor: withAlpha(surfaces.dark[0], 0.72),
  },
  title: {
    ...typeScale.base,
    fontWeight: "600",
    color: textAlphas.dark[1],
  },
  detail: {
    ...typeScale.sm,
    color: textAlphas.dark[2],
  },
  names: {
    ...typeScale.xs,
    color: textAlphas.dark[3],
  },
});
