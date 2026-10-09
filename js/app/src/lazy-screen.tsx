import { Loader, zero } from "@streamplace/components";
import type { ComponentProps, ComponentType } from "react";
import { lazy, Suspense } from "react";
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
  const Screen = lazy(load);
  return function LazyScreen(props: ComponentProps<T>) {
    return (
      <Suspense fallback={<ScreenFallback />}>
        <Screen {...props} />
      </Suspense>
    );
  };
}
