import { AlertCircle, CircleCheck, CircleX } from "lucide-react-native";
import { useSegmentTiming } from "../../../hooks/useSegmentTiming";
import * as atoms from "../../../lib/theme/atoms";
import { Text, View } from "../../ui";

type MetricsPanelProps = {
  showMetrics: boolean;
};

export function MetricsPanel({ showMetrics }: MetricsPanelProps) {
  const { connectionQuality, segmentDeltas, mean, range } = useSegmentTiming();

  let icon = <CircleX color="#d44" />;
  let color = "#d44";
  if (connectionQuality === "pre-live") {
    icon = <CircleCheck color={atoms.colors.blue[500]} />;
    color = atoms.colors.blue[500];
  } else if (connectionQuality === "good") {
    icon = <CircleCheck color="#4d4" />;
    color = "#4d4";
  } else if (connectionQuality === "degraded") {
    icon = <AlertCircle color="#aa4" />;
    color = "#aa4";
  } else {
    icon = <CircleX color="#d44" />;
    color = "#d44";
  }

  const connectionText = () => {
    if (connectionQuality === "pre-live") {
      return "READY TO STREAM";
    } else if (connectionQuality === "good") {
      return "GOOD";
    } else if (connectionQuality === "degraded") {
      return "DEGRADED";
    } else {
      return "POOR";
    }
  };

  return (
    <View
      style={{
        alignItems: "center",
        gap: 8,
      }}
    >
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          padding: 10,
          backgroundColor: "rgba(0, 0, 0, 0.5)",
          borderRadius: 8,
          gap: 4,
        }}
      >
        {icon}
        <Text
          style={[
            atoms.pt[0],
            {
              color,
            },
          ]}
        >
          {connectionText()}
        </Text>
      </View>
      {showMetrics && (
        <View>
          <Text>
            last Δ:{" "}
            {segmentDeltas.length > 0
              ? segmentDeltas[segmentDeltas.length - 1]
              : "—"}
          </Text>
          <Text>mean: {mean}</Text>
          <Text>range: {range}</Text>
        </View>
      )}
    </View>
  );
}
