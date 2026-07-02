import { MenuItem, Text, View } from "@streamplace/components";
import { Pressable, Switch, ViewStyle } from "react-native";

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
        <Switch value={value} onValueChange={onValueChange} />
      </Pressable>
    </MenuItem>
  );
}
