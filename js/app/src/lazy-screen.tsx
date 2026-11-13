import {
  AppCrashScreen,
  ErrorBoundary,
  Loader,
  zero,
} from "@streamplace/components";
import type { ComponentProps, ComponentType } from "react";
import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { View } from "react-native";

function ScreenFallback() {
  return (
    <View style={[zero.flex.values[1], zero.layout.flex.center]}>
      <Loader size="large" accessibilityLabel="Loading screen" />
    </View>
  );
}

async function reloadStaleScreen(signal: AbortSignal) {
  if (signal.aborted) return;
  const manifest = document.getElementById("bootstrap-scripts")?.textContent;
  if (!manifest) return;
  const destination = window.location.href;
  try {
    const response = await fetch(destination, { cache: "no-store", signal });
    if (
      !response.ok ||
      !response.headers.get("content-type")?.includes("text/html")
    )
      return;
    const html = new DOMParser().parseFromString(
      await response.text(),
      "text/html",
    );
    const latest = html.getElementById("bootstrap-scripts")?.textContent;
    // Only a changed build warrants a reload; outages keep the retry UI.
    if (
      !signal.aborted &&
      latest &&
      latest !== manifest &&
      window.location.href === destination
    ) {
      window.location.reload();
    }
  } catch {
    // Offline or unavailable HTML must leave navigation and manual retry usable.
  }
}

// Code-split a screen: its module loads on first render, behind a Suspense
// boundary local to the screen so the surrounding navigation stays usable.
export function lazyScreen<T extends ComponentType<any>>(
  load: () => Promise<{ default: T }>,
) {
  const InitialScreen = lazy(load);
  return function LazyScreen(props: ComponentProps<T>) {
    const [Screen, setScreen] = useState(() => InitialScreen);
    const deploymentCheck = useRef<AbortController | null>(null);
    useEffect(() => () => deploymentCheck.current?.abort(), []);
    return (
      <ErrorBoundary
        onError={() => {
          if (typeof window === "undefined" || typeof document === "undefined")
            return;
          deploymentCheck.current?.abort();
          const pending = new AbortController();
          deploymentCheck.current = pending;
          // Navigation commits its destination URL after the failed render.
          setTimeout(() => void reloadStaleScreen(pending.signal), 0);
        }}
        fallback={(reset) => (
          <AppCrashScreen
            reset={() => {
              deploymentCheck.current?.abort();
              // React.lazy caches rejection; a retry needs a fresh lazy component.
              setScreen(() => lazy(load));
              reset();
            }}
          />
        )}
      >
        <Suspense fallback={<ScreenFallback />}>
          <Screen {...props} />
        </Suspense>
      </ErrorBoundary>
    );
  };
}
