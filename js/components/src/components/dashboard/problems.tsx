import {
  CircleAlert,
  CircleX,
  ExternalLink,
  Info,
  Sparkle,
} from "lucide-react-native";
import { forwardRef, useImperativeHandle, useState } from "react";
import { Linking, Pressable, View } from "react-native";
import { statusColors, textAlphas } from "../../lib/theme/tokens";
import { LivestreamProblem, useLivestreamStore } from "../../livestream-store";
import * as zero from "../../ui";
import { Button, Text, useTheme } from "../ui";

const { r, p, layout, gap } = zero;

// Linear-style severity: colored icon, no heavy tinted fills
const getIcon = (severity: string) => {
  switch (severity) {
    case "error":
      return <CircleX size={20} color={statusColors.dark.danger} />;
    case "warning":
      return <CircleAlert size={20} color={statusColors.dark.warning} />;
    case "info":
      return <Info size={20} color={textAlphas.dark[2]} />;
    default:
      return <Sparkle size={20} color={textAlphas.dark[2]} />;
  }
};

const Problems = ({
  probs,
  onIgnore,
}: {
  probs: LivestreamProblem[];
  onIgnore: () => void;
}) => {
  const { theme } = useTheme();
  return (
    <View style={[gap.all[4]]}>
      <View style={[gap.all[2]]}>
        <Text size="xl" weight="semibold">
          Optimize Your Stream
        </Text>
        <Text size="sm" color="muted">
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
            <View style={[zero.p[1]]}>{getIcon(p.severity)}</View>
            <View style={[{ flex: 1 }, gap.all[1]]}>
              <Text weight="semibold">{p.code}</Text>
              <Text size="sm" color="muted">
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
                    <Text size="sm" style={{ color: theme.colors.primary }}>
                      Learn More
                    </Text>
                    <ExternalLink size={12} color={theme.colors.primary} />
                  </View>
                </Pressable>
              )}
            </View>
          </View>
        </View>
      ))}
      <View style={[layout.flex.row, layout.flex.justify.end]}>
        <Button onPress={onIgnore} variant="secondary" width="min">
          Acknowledge
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
  const { theme } = useTheme();
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
              backgroundColor: theme.colors.overlay,
              zIndex: 100,
            },
            layout.flex.center,
            { justifyContent: "flex-start" },
            p[12],
          ]}
        >
          <View
            style={[
              r.lg,
              p[8],
              {
                backgroundColor: theme.colors.surface2,
                borderWidth: 1,
                borderColor: theme.colors.borderSubtle,
                maxWidth: 700,
                width: "100%",
              },
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
