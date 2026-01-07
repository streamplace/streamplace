import { forwardRef, type ComponentProps } from "react";
import { StyleSheet } from "react-native";
import { useTheme, zero } from "../..";
import { LICENSE_URL_LABELS } from "../../lib/metadata-constants";
import { Text } from "../ui/text";

const { layout, gap, mt, text: textStyles } = zero;

export interface ContentRightsProps extends ComponentProps<typeof Text> {
  contentRights: {
    creator?: string;
    copyrightNotice?: string;
    copyrightYear?: string | number;
    license?: string;
    creditLine?: string;
  };
  compact?: boolean;
}

export const ContentRights = forwardRef<Text, ContentRightsProps>(
  ({ contentRights, compact, ...rest }, ref) => {
    const { zero } = useTheme();
    if (!contentRights || Object.keys(contentRights).length === 0) {
      return null;
    }

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
      elements.push(
        `© ${contentRights.copyrightYear.toString()}${contentRights.creditLine ? " " + contentRights.creditLine : ""}`,
      );
    } else if (contentRights.creditLine) {
      elements.push(contentRights.creditLine);
    }

    if (contentRights.license) {
      elements.push(formatLicense(contentRights.license));
    }

    if (contentRights.copyrightNotice) {
      elements.push(contentRights.copyrightNotice);
    }

    if (elements.length > 0) {
      elements[0] = "Stream content is " + elements[0];
    }

    if (elements.length == 0) {
      return null;
    }

    return (
      <Text
        ref={ref}
        style={[
          zero.text.mutedForeground,
          mt[1],
          StyleSheet.flatten(rest.style),
        ]}
        {...rest}
      >
        {elements.join(" • ")}
      </Text>
    );
  },
);

ContentRights.displayName = "ContentRights";
