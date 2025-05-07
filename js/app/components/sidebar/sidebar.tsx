import { YStack, styled, Text, View, Image } from "tamagui";
import { SharedValue, useAnimatedStyle } from "react-native-reanimated"; // Import SharedValue
import SidebarItem from "./sidebar-item";
import {
  CommonActions,
  DrawerNavigationState,
  ParamListBase,
} from "@react-navigation/native"; // Import necessary types
import { DrawerNavigationOptions } from "@react-navigation/drawer";
import { DrawerDescriptorMap } from "@react-navigation/drawer/lib/typescript/src/types";
import BreakpointIndicator from "components/debug/breakpoint-indicator";

const AnimatedYStack = styled(YStack, {
  name: "AnimatedYStack",
});

interface CustomSidebarProps {
  collapsed: boolean;
  widthAnim: SharedValue<number>;
  descriptors: DrawerDescriptorMap;
  state: DrawerNavigationState<ParamListBase>;
}

// Combine standard drawer props with custom props
type SidebarProps = CustomSidebarProps & DrawerNavigationOptions;

export default function Sidebar({
  state,
  descriptors,
  collapsed,
  widthAnim,
}: SidebarProps) {
  // Apply the defined type to the component props
  const animatedSidebarStyle = useAnimatedStyle(() => {
    return {
      minWidth: widthAnim.value,
      maxWidth: widthAnim.value,
    };
  });

  return (
    <AnimatedYStack
      style={animatedSidebarStyle} // Apply the animated style
      padding="$2"
      gap="$2"
    >
      <View
        marginTop="$3"
        marginBottom="$5"
        paddingLeft="$2.5"
        gap="$3"
        flexDirection="row"
        justifyContent="flex-start"
        alignItems="center"
      >
        {/* if we are in not production put the indicator here */}
        {__DEV__ ? (
          <BreakpointIndicator />
        ) : (
          <Image
            source={require("../../assets/images/cube.png")}
            height="$2"
            width="$2"
          />
        )}
        {!collapsed && <Text fontSize="$7">Streamplace</Text>}
      </View>

      {state.routes.map((route) => {
        const descriptor = descriptors[route.key];
        const options = descriptor?.options ?? {};
        if (options?.headerShown == false) {
          return null;
        }

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
            icon={IconComponent ? IconComponent : null}
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
            tint={options.drawerActiveTintColor as string | undefined} // Assuming tint is a string color or undefined
          />
        );
      })}
    </AnimatedYStack>
  );
}
