import type { ReactNode } from "react";
import { View } from "react-native";
import { gap, py } from "../../lib/theme/atoms";
import { ErrorBoundary } from "../error-boundary";
import { Button } from "./button";
import { Text } from "./text";

type Level = {
  key: string;
  content: ReactNode | ((state: { pressed: boolean }) => ReactNode);
};

function DropdownLevels({ stack }: { stack: Level[] }) {
  return stack.map((level, index) => (
    <View
      key={level.key}
      style={{ display: index === stack.length - 1 ? "flex" : "none" }}
    >
      {typeof level.content === "function"
        ? level.content({ pressed: true })
        : level.content}
    </View>
  ));
}

export function DropdownContent({
  stack,
  onDismiss,
}: {
  stack: Level[];
  onDismiss: () => void;
}) {
  return (
    <ErrorBoundary
      fallback={(reset) => (
        <View style={[gap.all[3], py[4]]} testID="menu-error">
          <Text>This menu couldn't be displayed.</Text>
          <Button onPress={reset}>
            <Text>Try again</Text>
          </Button>
          <Button variant="secondary" onPress={onDismiss}>
            <Text>Dismiss</Text>
          </Button>
        </View>
      )}
    >
      {/* Invoke render callbacks below the boundary too, not while constructing
          its children in the parent render. */}
      <DropdownLevels stack={stack} />
    </ErrorBoundary>
  );
}
