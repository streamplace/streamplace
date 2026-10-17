import { useNavigation } from "@react-navigation/native";
import { Text, useTheme, View, zero } from "@streamplace/components";
import { spacing } from "@streamplace/components/src/lib/theme/tokens";
import { ChevronRight, ExternalLink, LucideIcon } from "lucide-react-native";
import { Linking, Pressable } from "react-native";

interface SettingsNavigationItemProps {
  title: string;
  screen: string;
  icon: LucideIcon;
}

// Linear-style settings rows: quiet at rest, surface fill on hover/press,
// text3 icons, base-size labels.
function useRowStyles() {
  const { theme } = useTheme();
  return {
    row: (active: boolean) => [
      zero.px[3],
      zero.py[2],
      zero.layout.flex.row,
      zero.layout.flex.justify.between,
      zero.layout.flex.align.center,
      zero.r.md,
      {
        minHeight: 44,
        backgroundColor: active ? theme.colors.surface2 : "transparent",
      },
    ],
    iconColor: theme.colors.text3,
    chevronColor: theme.colors.text4,
  };
}

export function SettingsNavigationItem({
  title,
  screen,
  icon: Icon,
}: SettingsNavigationItemProps) {
  const navigation = useNavigation();
  const styles = useRowStyles();

  return (
    <Pressable onPress={() => navigation.navigate(screen as never)}>
      {({ pressed, hovered }: any) => (
        <View style={styles.row(pressed || hovered)}>
          <View
            style={{
              flexDirection: "row",
              alignItems: "center",
              gap: spacing[3],
            }}
          >
            <Icon size={18} color={styles.iconColor} />
            <Text>{title}</Text>
          </View>
          <ChevronRight size={18} color={styles.chevronColor} />
        </View>
      )}
    </Pressable>
  );
}

interface SettingsRowItemProps {
  children?: React.ReactNode;
  onPress?: () => void;
}

export function SettingsRowItem({ children, onPress }: SettingsRowItemProps) {
  const styles = useRowStyles();
  return (
    <Pressable onPress={onPress} style={{ width: "100%" }}>
      {({ pressed, hovered }: any) => (
        <View
          style={[
            ...styles.row(!!onPress && (pressed || hovered)),
            zero.w.percent[100],
          ]}
        >
          {children}
        </View>
      )}
    </Pressable>
  );
}

interface SettingsExternalItemProps {
  LeftIcon?: LucideIcon;
  title: string;
  link: string;
}

export function SettingsExternalItem({
  LeftIcon,
  title,
  link,
}: SettingsExternalItemProps) {
  // Cast LeftIcon to any to avoid type incompatibilities with ForwardRefExoticComponent
  const Left = LeftIcon as any;
  const styles = useRowStyles();

  return (
    <Pressable onPress={() => Linking.openURL(link)}>
      {({ pressed, hovered }: any) => (
        <View style={styles.row(pressed || hovered)}>
          <View
            style={{
              flexDirection: "row",
              alignItems: "center",
              gap: spacing[3],
            }}
          >
            {LeftIcon && <Left size={18} color={styles.iconColor} />}
            <Text>{title}</Text>
          </View>
          <ExternalLink size={18} color={styles.chevronColor} />
        </View>
      )}
    </Pressable>
  );
}
