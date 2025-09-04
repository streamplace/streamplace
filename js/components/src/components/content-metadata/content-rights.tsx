import { forwardRef } from "react";
import { StyleSheet, View } from "react-native";
import { LICENSE_URL_LABELS } from "../../lib/metadata-constants";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "../ui/text";

export interface ContentRightsProps {
  contentRights: {
    creator?: string;
    copyrightNotice?: string;
    copyrightYear?: string | number;
    license?: string;
    creditLine?: string;
  };
  compact?: boolean;
}

export const ContentRights = forwardRef<any, ContentRightsProps>(
  ({ contentRights }, ref) => {
    const { theme } = useTheme();

    if (!contentRights || Object.keys(contentRights).length === 0) {
      return null;
    }

    const styles = createStyles(theme);

    const formatLicense = (license: string) => {
      return LICENSE_URL_LABELS[license] || license;
    };

    // Display rights in bottom metadata view
    const elements: string[] = [];

    // TODO: Map DID to handle creator
    // if (contentRights.creator) {
    //   elements.push(`Creator: ${contentRights.creator}`);
    // }

    if (contentRights.copyrightYear) {
      elements.push(`© ${contentRights.copyrightYear.toString()}`);
    }

    if (contentRights.license) {
      elements.push(formatLicense(contentRights.license));
    }

    if (contentRights.copyrightNotice) {
      elements.push(contentRights.copyrightNotice);
    }

    if (contentRights.creditLine) {
      elements.push(contentRights.creditLine);
    }

    return (
      <View ref={ref} style={styles.compactContainer}>
        <Text style={styles.compactText}>{elements.join(" • ")}</Text>
      </View>
    );
  },
);

ContentRights.displayName = "ContentRights";

function createStyles(theme: any) {
  return StyleSheet.create({
    container: {
      paddingVertical: theme.spacing[3],
    },
    title: {
      fontSize: 14,
      fontWeight: "600",
      color: theme.colors.text,
      marginBottom: theme.spacing[2],
    },
    content: {
      gap: theme.spacing[2],
    },
    row: {
      flexDirection: "row",
      gap: theme.spacing[2],
    },
    label: {
      fontSize: 13,
      color: theme.colors.textMuted,
    },
    value: {
      fontSize: 13,
      color: theme.colors.text,
    },
    compactContainer: {
      flexDirection: "row",
      gap: theme.spacing[2],
      flexWrap: "wrap",
      marginTop: theme.spacing[1],
    },
    compactText: {
      fontSize: 12,
      color: theme.colors.textMuted,
    },
  });
}
