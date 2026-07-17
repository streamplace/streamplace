import { Text, useTheme } from "@streamplace/components";
import { ReactNode } from "react";
import { View } from "react-native";

/**
 * A recessed 64px tile holding a lucide icon — the standard illustration for
 * icon-based empty states (pass as `illustration` to <EmptyState>).
 */
export function EmptyStateTile({
  icon: Icon,
  tone,
}: {
  icon: React.ComponentType<any>;
  tone?: string;
}) {
  const { theme } = useTheme();
  const c = theme.colors;
  return (
    <View
      style={{
        width: 64,
        height: 64,
        borderRadius: 16,
        alignItems: "center",
        justifyContent: "center",
        backgroundColor: c.surface1,
        borderWidth: 1,
        borderColor: c.borderSubtle,
      }}
    >
      <Icon size={28} color={tone ?? c.text3} />
    </View>
  );
}

/**
 * Shared empty-state layout: a ~64px illustration, a semibold headline, muted
 * supporting copy, and an optional call to action — centered in the available
 * space. Used across Home, Videos, and the content hub so they read as one
 * system. Give it a parent with a flex context (e.g. a flex:1 column or a
 * flexGrow scroll container) for it to center vertically.
 */
export function EmptyState({
  illustration,
  title,
  subtitle,
  action,
}: {
  illustration: ReactNode;
  title: string;
  subtitle?: string;
  action?: ReactNode;
}) {
  return (
    <View
      style={{
        flex: 1,
        minHeight: 320,
        justifyContent: "center",
        alignItems: "center",
        paddingVertical: 48,
        paddingHorizontal: 24,
      }}
    >
      {illustration}
      <Text
        size="xl"
        weight="semibold"
        style={{ marginTop: 20, textAlign: "center" }}
      >
        {title}
      </Text>
      {subtitle ? (
        <Text
          color="muted"
          style={{
            marginTop: 8,
            textAlign: "center",
            maxWidth: 380,
            lineHeight: 21,
          }}
        >
          {subtitle}
        </Text>
      ) : null}
      {action ? <View style={{ marginTop: 24 }}>{action}</View> : null}
    </View>
  );
}
