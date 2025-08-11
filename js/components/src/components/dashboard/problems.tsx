import { ExternalLink } from "lucide-react-native";
import { useState } from "react";
import { Linking, Pressable, Text, View } from "react-native";
import { useLivestreamStore } from "../../livestream-store";
import { LivestreamProblem } from "../../livestream-store/livestream-state";
import * as zero from "../../ui";

const { bg, r, borders, p, text, layout, gap } = zero;

const Problems = ({
  probs,
  onIgnore,
}: {
  probs: LivestreamProblem[];
  onIgnore: () => void;
}) => {
  return (
    <View style={[gap.all[3]]}>
      <View>
        <Text style={[text.white, { fontSize: 24, fontWeight: "bold" }]}>
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
            <Text
              style={[
                r.sm,
                p[2],
                {
                  width: 82,
                  textAlign: "center",
                  backgroundColor:
                    p.severity === "error"
                      ? "#7f1d1d"
                      : p.severity === "warning"
                        ? "#7c2d12"
                        : "#1e3a8a",
                  color: "white",
                  fontSize: 12,
                },
              ]}
            >
              {p.severity}
            </Text>
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

      <Pressable
        onPress={onIgnore}
        style={[
          bg.blue[600],
          r.md,
          p[3],
          layout.flex.center,
          { marginTop: 16 },
        ]}
      >
        <Text style={[text.white, { fontWeight: "600" }]}>Ignore</Text>
      </Pressable>
    </View>
  );
};

export const ProblemsWrapper = ({
  children,
}: {
  children: React.ReactElement;
}) => {
  const problems = useLivestreamStore((x) => x.problems);
  const [dismiss, setDismiss] = useState(false);

  return (
    <View
      style={[
        { position: "relative", flex: 1 },
        layout.flex.center,
        { flexBasis: 0 },
      ]}
    >
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
            p[8],
          ]}
        >
          <View
            style={[
              bg.gray[900],
              borders.color.gray[700],
              borders.width.thin,
              r.lg,
              p[4],
              { maxWidth: 700, width: "100%" },
            ]}
          >
            <Problems probs={problems} onIgnore={() => setDismiss(true)} />
          </View>
        </View>
      )}
    </View>
  );
};

export default Problems;
