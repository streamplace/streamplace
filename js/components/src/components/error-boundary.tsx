import * as React from "react";
import { Platform, Pressable, Text, View } from "react-native";
import { darkTheme } from "../lib/theme/theme";
import { breakpoints, fontWeights, typeScale } from "../lib/theme/tokens";

type ErrorBoundaryProps = {
  children: React.ReactNode;
  /** What to show instead of the crashed subtree; `reset` retries the
   *  render. Default: a small "Error" label, for boundaries around one row
   *  or panel. */
  fallback?: React.ReactNode | ((reset: () => void) => React.ReactNode);
  onError?: (error: any, info: any) => void;
};

type ErrorBoundaryState = {
  hasError: boolean;
};

export class ErrorBoundary extends React.Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: any): ErrorBoundaryState {
    // Update state so the next render will show the fallback UI.
    return { hasError: true };
  }

  componentDidCatch(error: any, info: any) {
    console.error("<ErrorBoundary> caught:", error);
    this.props.onError?.(error, info);
  }

  reset = () => this.setState({ hasError: false });

  render() {
    if (this.state.hasError) {
      const { fallback } = this.props;
      if (typeof fallback === "function") return fallback(this.reset);
      if (fallback !== undefined) return fallback;
      return (
        <View>
          <Text>Error</Text>
        </View>
      );
    }

    return this.props.children;
  }
}

const theme = darkTheme;
// The root boundary sits outside FontProvider, so keep the system font family.
const bodyText = {
  fontSize: typeScale.base.fontSize,
  lineHeight: typeScale.base.lineHeight,
};
const button = {
  minHeight: theme.touchTargets.minimum,
  justifyContent: "center",
  alignItems: "center",
  paddingHorizontal: theme.spacing[4],
  borderRadius: theme.borderRadius.md,
  ...(Platform.OS === "web"
    ? {
        outlineColor: theme.colors.focus,
        outlineWidth: theme.spacing[1] / 2,
        outlineOffset: theme.spacing[1] / 2,
      }
    : {}),
} as const;

// Static theme tokens and native primitives only: app providers may have crashed.
export function AppCrashScreen({ reset }: { reset: () => void }) {
  return (
    <View
      testID="app-crash-screen"
      style={{
        flex: 1,
        alignItems: "center",
        justifyContent: "center",
        padding: theme.spacing[6],
        backgroundColor: theme.colors.surface0,
      }}
    >
      <View
        style={{
          width: "100%",
          maxWidth: breakpoints.sm,
          gap: theme.spacing[6],
          padding: theme.spacing[8],
          borderWidth: 1,
          borderColor: theme.colors.borderSubtle,
          borderRadius: theme.borderRadius.lg,
          backgroundColor: theme.colors.surface1,
        }}
      >
        <Text
          style={{
            fontSize: typeScale.md.fontSize,
            lineHeight: typeScale.md.lineHeight,
            fontWeight: fontWeights.semibold,
            color: theme.colors.text1,
          }}
        >
          Streamplace
        </Text>
        <View style={{ gap: theme.spacing[2] }}>
          <Text
            accessibilityRole="header"
            style={{
              fontSize: typeScale.xl.fontSize,
              lineHeight: typeScale.xl.lineHeight,
              letterSpacing: typeScale.xl.letterSpacing,
              fontWeight: fontWeights.semibold,
              color: theme.colors.text1,
            }}
          >
            Something went wrong
          </Text>
          <Text style={{ ...bodyText, color: theme.colors.text2 }}>
            This screen hit an error it couldn't recover from.
          </Text>
        </View>
        <View style={{ gap: theme.spacing[3] }}>
          <Pressable
            onPress={reset}
            accessibilityRole="button"
            style={[button, { backgroundColor: theme.colors.primary }]}
          >
            <Text
              style={{
                ...bodyText,
                fontWeight: fontWeights.medium,
                color: theme.colors.primaryForeground,
              }}
            >
              Try again
            </Text>
          </Pressable>
          {Platform.OS === "web" && (
            <Pressable
              onPress={() => window.location.reload()}
              accessibilityRole="button"
              style={[
                button,
                { borderWidth: 1, borderColor: theme.colors.border },
              ]}
            >
              <Text
                style={{
                  ...bodyText,
                  fontWeight: fontWeights.medium,
                  color: theme.colors.text1,
                }}
              >
                Reload the page
              </Text>
            </Pressable>
          )}
        </View>
      </View>
    </View>
  );
}
