import { forwardRef } from "react";
import { View } from "react-native";
import { zero } from "../..";
import { C2PA_WARNING_LABELS } from "../../lib/metadata-constants";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "../ui/text";

const { layout, gap, bg, r, p, px, py, text: textStyles, borders } = zero;

export interface ContentWarningsProps {
  warnings: string[];
  compact?: boolean;
}

export const ContentWarnings = forwardRef<View, ContentWarningsProps>(
  ({ warnings, compact = false, ...rest }, ref) => {
    const { theme } = useTheme();

    if (!warnings || warnings.length === 0) {
      return null;
    }

    const getWarningLabel = (warning: string): string => {
      return C2PA_WARNING_LABELS[warning] || warning;
    };

    if (compact) {
      return (
        <View
          ref={ref}
          style={[layout.flex.row, layout.flex.wrap.wrap, gap.all[1]]}
          {...rest}
        >
          {warnings.map((warning, index) => (
            <View
              key={index}
              style={[
                { backgroundColor: theme.colors.warning },
                r.full,
                px[2],
                { paddingVertical: 2 },
              ]}
            >
              <Text
                size="sm"
                style={[{ color: theme.colors.warningForeground }]}
              >
                {getWarningLabel(warning)}
              </Text>
            </View>
          ))}
        </View>
      );
    }

    return (
      <View ref={ref} style={[layout.flex.column, gap.all[2]]} {...rest}>
        <Text
          style={[{ fontSize: 14, fontWeight: "600" }, textStyles.gray[900]]}
        >
          Content Warnings
        </Text>
        <View style={[layout.flex.row, layout.flex.wrap.wrap, gap.all[2]]}>
          {warnings.map((warning, index) => (
            <View
              key={index}
              style={[
                { backgroundColor: theme.colors.warning },
                r.full,
                px[3],
                py[1],
              ]}
            >
              <Text
                size="sm"
                style={[{ color: theme.colors.warningForeground }]}
              >
                {getWarningLabel(warning)}
              </Text>
            </View>
          ))}
        </View>
      </View>
    );
  },
);

ContentWarnings.displayName = "ContentWarnings";
