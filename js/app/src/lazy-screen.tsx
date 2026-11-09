import {
  AppCrashScreen,
  ErrorBoundary,
  Loader,
  zero,
} from "@streamplace/components";
import type { ComponentProps, ComponentType } from "react";
import { lazy, Suspense, useState } from "react";
import { View } from "react-native";

function ScreenFallback() {
  return (
    <View style={[zero.flex.values[1], zero.layout.flex.center]}>
      <Loader size="large" accessibilityLabel="Loading screen" />
    </View>
  );
}

async function reloadStaleScreen() {
  if (typeof window === "undefined" || typeof document === "undefined") return;
  const manifest = document.getElementById("bootstrap-scripts")?.textContent;
  if (!manifest) return;
  const destination = window.location.href;
  try {
    const response = await fetch(destination, { cache: "no-store" });
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
    if (latest && latest !== manifest && window.location.href === destination) {
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
    return (
      <ErrorBoundary
        onError={() => {
          // Navigation commits its destination URL after the failed render.
          setTimeout(reloadStaleScreen, 0);
        }}
        fallback={(reset) => (
          <AppCrashScreen
            reset={() => {
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
