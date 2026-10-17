import { Text } from "@streamplace/components";
import { ReactNode } from "react";
import { View } from "react-native";

/**
 * The standard header for a settings management view: a semibold title, muted
 * supporting copy, and an optional primary action (Create/Add/Refresh) pinned
 * to the right. Shared across Keys, Recommendations, Webhooks, and Multistream
 * so every management view opens with the same rhythm.
 */
export function SettingsViewHeader({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <View
      style={{
        flexDirection: "row",
        justifyContent: "space-between",
        alignItems: "flex-start",
        gap: 16,
        marginBottom: 20,
      }}
    >
      {/* minWidth:0 is load-bearing: flex items default to min-width:auto on
          web, which stops this column shrinking below its min-content and
          shoves the (flexShrink:0) action group outside the container. */}
      <View style={{ flex: 1, minWidth: 0, gap: 8 }}>
        <Text size="xl" weight="semibold">
          {title}
        </Text>
        {description ? (
          <Text color="muted" style={{ lineHeight: 20, maxWidth: 560 }}>
            {description}
          </Text>
        ) : null}
      </View>
      {action ? <View style={{ flexShrink: 0 }}>{action}</View> : null}
    </View>
  );
}
