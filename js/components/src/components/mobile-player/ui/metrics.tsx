import { useSegmentTiming } from "../../../hooks/useSegmentTiming";
import {
  borderAlphas,
  borderRadius,
  fontFamilies,
  scrims,
  spacing,
  typeScale,
} from "../../../lib/theme/tokens";
import { Text, useTheme, View } from "../../ui";

type MetricsPanelProps = {
  showMetrics: boolean;
};

// Over-video hairline — the scrimmed pills sit on bright camera footage, so a
// faint white edge keeps them crisp without a hard border.
const HUD_BORDER = borderAlphas.dark.strong;

export function MetricsPanel({ showMetrics }: MetricsPanelProps) {
  const { connectionQuality, segmentDeltas, mean, range } = useSegmentTiming();
  const { theme } = useTheme();
  const c = theme.colors;

  let dotColor = c.danger;
  let label = "POOR";
  if (connectionQuality === "pre-live") {
    dotColor = c.primary;
    label = "READY TO STREAM";
  } else if (connectionQuality === "good") {
    dotColor = c.success;
    label = "GOOD";
  } else if (connectionQuality === "degraded") {
    dotColor = c.warning;
    label = "DEGRADED";
  } else {
    dotColor = c.danger;
    label = "POOR";
  }

  const debugRows: [string, string][] = [
    [
      "last Δ",
      segmentDeltas.length > 0
        ? `${segmentDeltas[segmentDeltas.length - 1]}ms`
        : "—",
    ],
    ["mean", mean != null ? `${mean}ms` : "—"],
    ["range", range != null ? `${range}ms` : "—"],
  ];

  return (
    <View style={{ alignItems: "flex-start", gap: spacing[1] }}>
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: spacing[2],
          paddingHorizontal: spacing[3],
          paddingVertical: 7,
          backgroundColor: scrims.dark,
          borderRadius: borderRadius.full,
          borderWidth: 1,
          borderColor: HUD_BORDER,
        }}
      >
        <View
          style={{
            width: 7,
            height: 7,
            borderRadius: borderRadius.full,
            backgroundColor: dotColor,
          }}
        />
        <Text
          style={{
            color: c.text,
            fontSize: typeScale.xs.fontSize,
            lineHeight: typeScale.xs.lineHeight,
            fontFamily: fontFamilies.semiBold,
            fontWeight: "600",
            letterSpacing: 0.5,
          }}
        >
          {label}
        </Text>
      </View>
      {showMetrics && (
        <View
          style={{
            paddingHorizontal: spacing[3],
            paddingVertical: spacing[2],
            backgroundColor: scrims.dark,
            borderRadius: borderRadius.md,
            borderWidth: 1,
            borderColor: HUD_BORDER,
            gap: 3,
            minWidth: 132,
          }}
        >
          {debugRows.map(([k, v]) => (
            <View
              key={k}
              style={{
                flexDirection: "row",
                justifyContent: "space-between",
                gap: spacing[4],
              }}
            >
              <Text
                style={{
                  color: c.text3,
                  fontSize: typeScale.xs.fontSize,
                  lineHeight: typeScale.xs.lineHeight,
                  fontFamily: fontFamilies.monoRegular,
                }}
              >
                {k}
              </Text>
              <Text
                style={{
                  color: c.text2,
                  fontSize: typeScale.xs.fontSize,
                  lineHeight: typeScale.xs.lineHeight,
                  fontFamily: fontFamilies.monoMedium,
                }}
              >
                {v}
              </Text>
            </View>
          ))}
        </View>
      )}
    </View>
  );
}
