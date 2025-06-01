import { DrawerNavigationOptions } from "@react-navigation/drawer";
import { DrawerDescriptorMap } from "@react-navigation/drawer/lib/typescript/src/types";
import {
  CommonActions,
  DrawerNavigationState,
  ParamListBase,
} from "@react-navigation/native";
import { FileQuestion } from "@tamagui/lucide-icons";
import { Platform } from "react-native";
import { SharedValue, useAnimatedStyle } from "react-native-reanimated";
import { Image, styled, Text, View, YStack } from "tamagui";
import SidebarItem from "./sidebar-item";

const AnimatedYStack = styled(YStack, {
  name: "AnimatedYStack",
});

export interface ExternalDrawerItem {
  item: React.NamedExoticComponent<any>;
  label: React.ComponentType<any> | string;
  onPress: () => void;
}

interface CustomSidebarProps {
  collapsed: boolean;
  hidden: boolean;
  widthAnim: SharedValue<number>;
  descriptors: DrawerDescriptorMap;
  state: DrawerNavigationState<ParamListBase>;
  externalItems?: ExternalDrawerItem[];
}

// Combine standard drawer props with custom props
type SidebarProps = CustomSidebarProps & DrawerNavigationOptions;

export default function Sidebar({
  state,
  descriptors,
  collapsed,
  hidden,
  widthAnim,
  externalItems = [],
}: SidebarProps) {
  // Apply the defined type to the component props
  const animatedSidebarStyle = useAnimatedStyle(() => {
    return {
      minWidth: widthAnim.value,
      maxWidth: widthAnim.value,
    };
  });

  if (hidden) {
    return <View />;
  }

  return (
    <AnimatedYStack
      style={animatedSidebarStyle} // Apply the animated style
      padding="$2"
      gap="$2"
    >
      <View
        marginTop={Platform.OS === "ios" ? 29 : 12}
        marginBottom="$5"
        paddingLeft="$2.5"
        gap="$3"
        flexDirection="row"
        justifyContent="flex-start"
        alignItems="center"
      >
        <Image
          source={require("../../assets/images/cube.png")}
          height="$2"
          width="$2"
        />
        {!collapsed && (
          <Text fontSize="$7" minWidth={200} numberOfLines={1}>
            Streamplace
          </Text>
        )}
      </View>

      {state.routes.map((route) => {
        const descriptor = descriptors[route.key];
        const options = descriptor?.options ?? {};

        const label =
          typeof options.drawerLabel === "function"
            ? options.drawerLabel({ focused: false, color: "$color" })
            : (options.drawerLabel ?? options.title ?? route.name);

        const IconComponent = options.drawerIcon as
          | React.ComponentType<any>
          | undefined;

        return (
          <SidebarItem
            key={route.key}
            icon={IconComponent ? IconComponent : FileQuestion}
            label={label}
            active={descriptor.navigation.isFocused()}
            collapsed={collapsed}
            onPress={() => {
              if (route.name === "Home") {
                // copy logic for 'Home' to reset the stack
                descriptor.navigation.dispatch(
                  CommonActions.reset({
                    index: 0,
                    routes: [
                      {
                        name: "Home",
                        state: {
                          routes: [{ name: "StreamList" }],
                        },
                      },
                    ],
                  }),
                );
              } else {
                descriptor.navigation.navigate(route.name);
              }
            }}
            style={options.drawerItemStyle}
            tint={options.drawerActiveTintColor as string} // Assuming tint is a string color or undefined
          />
        );
      })}
      {externalItems.map((i) => {
        return (
          <SidebarItem
            key={JSON.stringify(i.label)}
            icon={i.item}
            label={i.label || "Fix this label!"}
            active={false}
            collapsed={collapsed}
            onPress={() => i.onPress()}
            tint="rgba(189, 110, 134)"
          />
        );
      })}
    </AnimatedYStack>
  );
}
