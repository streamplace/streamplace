import { MenuItem, Text, View } from "@streamplace/components";
import { Switch, ViewStyle } from "react-native";

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
      <View style={{ flex: 1, paddingRight: 12 }}>
        <Text size="base">{title}</Text>
        {description && (
          <Text size="sm" color="muted" style={{ marginTop: 2 }}>
            {description}
          </Text>
        )}
      </View>
      <Switch value={value} onValueChange={onValueChange} />
    </MenuItem>
  );
}
