import {
  Switch as RNSwitch,
  type SwitchProps as RNSwitchProps,
} from "react-native";
import { useTheme } from "../../lib/theme/theme";

/**
 * The app's toggle. React Native's Switch ships with a platform teal/green
 * track, which is off-brand — this wraps it so "on" is the reserved indigo
 * (indigo = state) and "off" is a quiet raised surface. Use this everywhere
 * instead of importing Switch from react-native.
 */
export function Switch(props: RNSwitchProps) {
  const { theme } = useTheme();
  const c = theme.colors;
  return (
    <RNSwitch
      trackColor={{ false: c.surface3, true: c.primary }}
      thumbColor={c.inverse}
      ios_backgroundColor={c.surface3}
      // react-native-web drives the "on" state from its own
      // activeTrackColor/activeThumbColor props (not in RN's types) and
      // defaults them to Material teal — override both or the thumb stays teal.
      {...({
        activeTrackColor: c.primary,
        activeThumbColor: c.inverse,
      } as object)}
      {...props}
    />
  );
}
