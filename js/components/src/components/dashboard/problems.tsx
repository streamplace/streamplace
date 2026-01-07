import {
  CircleAlert,
  CircleX,
  ExternalLink,
  Info,
  Sparkle,
} from "lucide-react-native";
import { forwardRef, useImperativeHandle, useState } from "react";
import { Linking, Pressable, View } from "react-native";
import { useLivestreamStore } from "../../livestream-store";
import { LivestreamProblem } from "../../livestream-store/livestream-state";
import * as zero from "../../ui";
import { Button, Text } from "../ui";

const { bg, r, borders, p, text, layout, gap } = zero;

const getIcon = (severity: string) => {
  switch (severity) {
    case "error":
      return <CircleX size={24} color="white" />;
    case "warning":
      return <CircleAlert size={24} color="white" />;
    case "info":
      return <Info size={24} color="white" />;
    default:
      return <Sparkle size={24} color="white" />;
  }
};

const Problems = ({
  probs,
  onIgnore,
}: {
  probs: LivestreamProblem[];
  onIgnore: () => void;
}) => {
  return (
    <View style={[gap.all[4]]}>
      <View style={[gap.all[2]]}>
        <Text size="2xl" style={[text.white, { fontWeight: "600" }]}>
          Optimize Your Stream
        </Text>
        <Text style={[text.gray[300]]}>
          We've found a few things that could improve your stream's reliability.
        </Text>
      </View>
      {probs.map((p) => (
        <View key={p.message}>
          <View
            style={[
              gap.all[2],
              layout.flex.row,
              layout.flex.alignCenter,
              { gap: 8, alignItems: "flex-start" },
            ]}
          >
            <View
              style={[
                zero.r.full,
                zero.p[1],
                {
                  backgroundColor:
                    p.severity === "error"
                      ? "#7f1d1d"
                      : p.severity === "warning"
                        ? "#7c2d12"
                        : "#1e3a8a",
                },
              ]}
            >
              {getIcon(p.severity)}
            </View>
            <View style={[{ flex: 1 }, gap.all[1]]}>
              <Text style={[text.white, { fontWeight: "600" }]}>{p.code}</Text>
              <Text style={[text.gray[400], { fontSize: 14 }]}>
                {p.message}
              </Text>
              {p.link && (
                <Pressable onPress={() => p.link && Linking.openURL(p.link)}>
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      gap.all[2],
                    ]}
                  >
                    <Text style={[{ color: "#3b82f6", fontSize: 14 }]}>
                      Learn More
                    </Text>
                    <ExternalLink size={12} color="#3b82f6" />
                  </View>
                </Pressable>
              )}
            </View>
          </View>
        </View>
      ))}
      <View style={[layout.flex.row, layout.flex.justify.end]}>
        <Button onPress={onIgnore} variant="secondary">
          <Text style={[text.white, { fontWeight: "600" }]}>Acknowledge</Text>
        </Button>
      </View>
    </View>
  );
};

export interface ProblemsWrapperRef {
  setDismiss: (value: boolean) => void;
}

export const ProblemsWrapper = forwardRef<
  ProblemsWrapperRef,
  {
    children: React.ReactElement;
  }
>(({ children }, ref) => {
  const problems = useLivestreamStore((x) => x.problems);
  const [dismiss, setDismiss] = useState(false);

  useImperativeHandle(ref, () => ({
    setDismiss,
  }));

  return (
    <>
      {children}
      {problems.length > 0 && !dismiss && (
        <View
          style={[
            {
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              backgroundColor: "rgba(0, 0, 0, 0.8)",
              zIndex: 100,
            },
            layout.flex.center,
            { justifyContent: "flex-start" },
            p[12],
          ]}
        >
          <View
            style={[
              bg.neutral[900],
              borders.color.neutral[700],
              borders.width.thin,
              r.lg,
              p[8],
              { maxWidth: 700, width: "100%" },
            ]}
          >
            <Problems probs={problems} onIgnore={() => setDismiss(true)} />
          </View>
        </View>
      )}
    </>
  );
});

export default Problems;
