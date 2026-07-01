import { MenuItem, Text, View } from "@streamplace/components";
import { Pressable, Switch, ViewStyle } from "react-native";

export interface SettingToggleProps {
  title: string;
  description?: string;
  value: boolean;
  onValueChange: (value: boolean) => void;
  style?: ViewStyle;
}

export function SettingToggle({
  title,
  description,
  value,
  onValueChange,
  style,
}: SettingToggleProps) {
  return (
    <MenuItem style={style}>
      <Pressable
        style={{ flex: 1, flexDirection: "row", alignItems: "center" }}
        onPress={() => onValueChange(!value)}
        accessibilityRole="switch"
        accessibilityState={{ checked: value }}
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
