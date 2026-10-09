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
