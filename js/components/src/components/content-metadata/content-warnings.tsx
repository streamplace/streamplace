import { forwardRef } from "react";
import { StyleSheet, View } from "react-native";
import { C2PA_WARNING_LABELS } from "../../lib/metadata-constants";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "../ui/text";

export interface ContentWarningsProps {
  warnings: string[];
  compact?: boolean;
}

export const ContentWarnings = forwardRef<any, ContentWarningsProps>(
  ({ warnings, compact = false }, ref) => {
    const { theme } = useTheme();

    if (!warnings || warnings.length === 0) {
      return null;
    }

    const styles = createStyles(theme, compact);

    const getWarningLabel = (warning: string): string => {
      return C2PA_WARNING_LABELS[warning] || warning;
    };

    if (compact) {
      return (
        <View ref={ref} style={styles.compactContainer}>
          {warnings.map((warning, index) => (
            <View key={index} style={styles.compactWarning}>
              <Text style={styles.compactWarningText}>
                {getWarningLabel(warning)}
              </Text>
            </View>
          ))}
        </View>
      );
    }

    return (
      <View ref={ref} style={styles.container}>
        <Text style={styles.title}>Content Warnings</Text>
        <View style={styles.warningsContainer}>
          {warnings.map((warning, index) => (
            <View key={index} style={styles.warning}>
              <Text style={styles.warningText}>{getWarningLabel(warning)}</Text>
            </View>
          ))}
        </View>
      </View>
    );
  },
);

ContentWarnings.displayName = "ContentWarnings";

function createStyles(theme: any, compact: boolean) {
  return StyleSheet.create({
    container: {
      flexDirection: "column",
      gap: theme.spacing[2],
    },
    title: {
      fontSize: 14,
      fontWeight: "600",
      color: theme.colors.text,
    },
    warningsContainer: {
      flexDirection: "row",
      flexWrap: "wrap",
      gap: theme.spacing[2],
    },
    warning: {
      backgroundColor: theme.colors.warning,
      borderRadius: theme.borderRadius.md,
      padding: theme.spacing[2],
    },
    warningText: {
      color: theme.colors.warningForeground,
      fontSize: 12,
      fontWeight: "500",
    },
    compactContainer: {
      flexDirection: "row",
      flexWrap: "wrap",
      gap: theme.spacing[1],
    },
    compactWarning: {
      backgroundColor: theme.colors.warning,
      borderRadius: theme.borderRadius.full,
      paddingHorizontal: 6,
      paddingVertical: 2,
    },
    compactWarningText: {
      color: theme.colors.warningForeground,
      fontSize: 10,
      fontWeight: "500",
    },
  });
}
