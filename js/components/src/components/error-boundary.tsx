import * as React from "react";
import { Platform, Pressable, Text, View } from "react-native";
import { defaultChrome } from "../lib/theme/chrome";
import {
  borderRadius,
  breakpoints,
  colors,
  fontWeights,
  spacing,
  touchTargets,
  typeScale,
} from "../lib/theme/tokens";

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

const chrome = defaultChrome(true);
// eslint-disable-next-line streamplace/no-token-literals -- Provider-independent crash fallback.
const primary = colors.primary[500];
// The root boundary sits outside FontProvider, so keep the system font family.
const bodyText = {
  fontSize: typeScale.base.fontSize,
  lineHeight: typeScale.base.lineHeight,
};
const button = {
  minHeight: touchTargets.minimum,
  justifyContent: "center",
  alignItems: "center",
  paddingHorizontal: spacing[4],
  borderRadius: borderRadius.md,
  ...(Platform.OS === "web"
    ? {
        outlineColor: primary,
        outlineWidth: spacing[1] / 2,
        outlineOffset: spacing[1] / 2,
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
        padding: spacing[6],
        backgroundColor: chrome.surface[0],
      }}
    >
      <View
        style={{
          width: "100%",
          maxWidth: breakpoints.sm,
          gap: spacing[6],
          padding: spacing[8],
          borderWidth: 1,
          borderColor: chrome.border.subtle,
          borderRadius: borderRadius.lg,
          backgroundColor: chrome.surface[1],
        }}
      >
        <Text
          style={{
            fontSize: typeScale.md.fontSize,
            lineHeight: typeScale.md.lineHeight,
            fontWeight: fontWeights.semibold,
            color: chrome.text[1],
          }}
        >
          Streamplace
        </Text>
        <View style={{ gap: spacing[2] }}>
          <Text
            accessibilityRole="header"
            style={{
              fontSize: typeScale.xl.fontSize,
              lineHeight: typeScale.xl.lineHeight,
              letterSpacing: typeScale.xl.letterSpacing,
              fontWeight: fontWeights.semibold,
              color: chrome.text[1],
            }}
          >
            Something went wrong
          </Text>
          <Text style={{ ...bodyText, color: chrome.text[2] }}>
            This screen hit an error it couldn't recover from.
          </Text>
        </View>
        <View style={{ gap: spacing[3] }}>
          <Pressable
            onPress={reset}
            accessibilityRole="button"
            style={[button, { backgroundColor: primary }]}
          >
            <Text
              style={{
                ...bodyText,
                fontWeight: fontWeights.medium,
                color: colors.white,
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
                { borderWidth: 1, borderColor: chrome.border.default },
              ]}
            >
              <Text
                style={{
                  ...bodyText,
                  fontWeight: fontWeights.medium,
                  color: chrome.text[1],
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
