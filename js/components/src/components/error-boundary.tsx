import * as React from "react";
import { Platform, Pressable, Text, View } from "react-native";

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

/**
 * Whole-app fallback: a plain screen with a retry that re-renders the tree
 * and, on the web, a reload. Deliberately free of the app's own providers
 * and theme, since those may be what crashed.
 */
export function AppCrashScreen({ reset }: { reset: () => void }) {
  const reload = () => {
    if (Platform.OS === "web" && typeof window !== "undefined") {
      window.location.reload();
    } else {
      reset();
    }
  };
  return (
    <View
      style={{
        flex: 1,
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
        backgroundColor: "#161928", // token-ok: no theme available here
      }}
    >
      <Text
        style={{
          color: "#ffffff", // token-ok
          fontSize: 20,
          fontWeight: "600",
          marginBottom: 8,
          textAlign: "center",
        }}
      >
        Something went wrong
      </Text>
      <Text
        style={{
          color: "#d0cfd7", // token-ok
          fontSize: 15,
          marginBottom: 20,
          textAlign: "center",
        }}
      >
        The page hit an error it couldn't recover from.
      </Text>
      <Pressable
        onPress={reset}
        accessibilityRole="button"
        style={{
          paddingVertical: 10,
          paddingHorizontal: 20,
          borderRadius: 999,
          backgroundColor: "#272f43", // token-ok
          marginBottom: 10,
        }}
      >
        <Text style={{ color: "#ffffff", fontSize: 15 }}>Try again</Text>
      </Pressable>
      <Pressable onPress={reload} accessibilityRole="button">
        <Text style={{ color: "#11e8b2", fontSize: 15 }}>Reload the page</Text>
      </Pressable>
    </View>
  );
}
