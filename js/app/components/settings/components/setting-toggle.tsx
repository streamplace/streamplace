import { MenuItem, Switch, Text, View } from "@streamplace/components";
import { Pressable, ViewStyle } from "react-native";

export interface SettingToggleProps {
  title: string;
  description?: string;
  value: boolean;
  onValueChange: (value: boolean) => void;
  style?: ViewStyle;
  testID?: string;
}

export function SettingToggle({
  title,
  description,
  value,
  onValueChange,
  style,
  testID,
}: SettingToggleProps) {
  return (
    <MenuItem style={style}>
      <Pressable
        style={{ flex: 1, flexDirection: "row", alignItems: "center" }}
        onPress={() => onValueChange(!value)}
        accessibilityRole="switch"
        // accessibilityRole="switch" collapses the subtree into one element
        // on iOS, hiding the title from Maestro's text matcher. Expose the
        // title as the element's label and a stable testID so e2e can find
        // it by id on both platforms.
        accessibilityLabel={title}
        accessibilityState={{ checked: value }}
        testID={testID}
      >
        <View style={{ flex: 1, paddingRight: 12 }}>
          <Text size="base">{title}</Text>
          {description && (
            <Text size="sm" color="muted" style={{ marginTop: 2 }}>
              {description}
            </Text>
          )}
        </View>
        {/* The row is the only control. A click on the nested Switch bubbles
            to the Pressable as well, so an interactive Switch fires
            onValueChange twice for one tap — once from the switch's change
            event and once from the row's press — duplicating whatever side
            effect the caller hangs off it. Inert here; the row still
            toggles, by tap or by keyboard. */}
        <View pointerEvents="none">
          <Switch value={value} />
        </View>
      </Pressable>
    </MenuItem>
  );
}
