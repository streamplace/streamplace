import { useNavigation } from "@react-navigation/native";
import { Text, View, zero } from "@streamplace/components";
import { ChevronRight, ExternalLink, LucideIcon } from "lucide-react-native";
import { Linking, Pressable } from "react-native";

interface SettingsNavigationItemProps {
  title: string;
  screen: string;
  icon: LucideIcon;
}

export function SettingsNavigationItem({
  title,
  screen,
  icon: Icon,
}: SettingsNavigationItemProps) {
  const navigation = useNavigation();

  return (
    <Pressable onPress={() => navigation.navigate(screen as never)}>
      {({ pressed }) => (
        <View
          style={[
            zero.px[3],
            zero.py[2],
            zero.layout.flex.row,
            zero.layout.flex.justify.between,
            zero.layout.flex.align.center,
            zero.r.md,
            {
              backgroundColor: pressed ? "#ffffff08" : "transparent",
            },
          ]}
        >
          <View style={{ flexDirection: "row", alignItems: "center", gap: 12 }}>
            <Icon size={20} color="#999" />
            <Text size="lg">{title}</Text>
          </View>
          <ChevronRight size={20} color="#666" />
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
  return (
    <Pressable onPress={onPress}>
      {({ pressed }) => (
        <View
          style={[
            zero.px[3],
            zero.py[2],
            zero.layout.flex.row,
            zero.layout.flex.justify.between,
            zero.layout.flex.align.center,
            zero.r.md,
            {
              backgroundColor: pressed && onPress ? "#ffffff08" : "transparent",
            },
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

  return (
    <Pressable onPress={() => Linking.openURL(link)}>
      {({ pressed }) => (
        <View
          style={[
            zero.px[3],
            zero.py[2],
            zero.layout.flex.row,
            zero.layout.flex.justify.between,
            zero.layout.flex.align.center,
            zero.r.md,
            {
              backgroundColor: pressed ? "#ffffff08" : "transparent",
            },
          ]}
        >
          <View style={{ flexDirection: "row", alignItems: "center", gap: 12 }}>
            {LeftIcon && <Left size={20} color="#999" />}
            <Text size="lg">{title}</Text>
          </View>
          <ExternalLink size={20} color="#666" />
        </View>
      )}
    </Pressable>
  );
}
