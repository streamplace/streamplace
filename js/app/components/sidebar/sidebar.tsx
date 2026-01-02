import { DrawerNavigationOptions } from "@react-navigation/drawer";
import { DrawerDescriptorMap } from "@react-navigation/drawer/lib/typescript/src/types";
import {
  CommonActions,
  DrawerNavigationState,
  ParamListBase,
  useNavigation,
} from "@react-navigation/native";
import { Text, zero } from "@streamplace/components";
import { useAQLinkHref } from "components/aqlink";
import React from "react";
import { Image, Platform, Pressable, View } from "react-native";
import Animated, {
  SharedValue,
  useAnimatedStyle,
} from "react-native-reanimated";
import SidebarItem from "./sidebar-item";

export interface ExternalDrawerItem {
  item:
    | React.ComponentType<any>
    | React.ReactElement
    | (() => React.ReactElement);
  label: React.ComponentType<any> | React.ReactElement | string;
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
  const navigation = useNavigation();
  const animatedSidebarStyle = useAnimatedStyle(() => {
    return {
      minWidth: widthAnim.value,
      maxWidth: widthAnim.value,
    };
  });

  // home route
  const route = state.routes.find((r) => r.name === "Home");
  const { href } = useAQLinkHref({
    screen: route ? route.name : "Home",
    params: route?.params as any,
  });

  if (hidden) {
    return <View />;
  }

  return (
    <Animated.View
      style={[
        animatedSidebarStyle,
        zero.p[2],
        zero.gap.all[2],
        zero.layout.flex.column,
      ]}
    >
      <Pressable
        // @ts-ignore This makes it render as <a> on web!
        href={route ? href : undefined}
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.gap.all[3],
          {
            marginTop: Platform.OS === "ios" ? 29 : 8,
            marginBottom: 20,
            paddingLeft: 11,
          },
        ]}
      >
        <Image
          source={require("../../assets/images/cube.png")}
          height={30}
          width={28}
          style={{ width: 28, height: 30, resizeMode: "contain" }}
        />
        {!collapsed && <Text size="2xl">Streamplace</Text>}
      </Pressable>

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

        // if we have style display: none on the drawer item, completely skip rendering it
        const drawerItemStyle = options.drawerItemStyle;
        let isHidden = false;
        if (
          drawerItemStyle &&
          typeof drawerItemStyle === "object" &&
          "display" in drawerItemStyle &&
          (drawerItemStyle as any).display === "none"
        ) {
          isHidden = true;
        }
        if (isHidden) {
          return null;
        }

        return (
          <SidebarItem
            key={route.key}
            icon={IconComponent ? IconComponent : () => <Text>📄</Text>}
            label={label}
            active={descriptor.navigation.isFocused()}
            collapsed={collapsed}
            route={route}
            onPress={(ev) => {
              ev.preventDefault();
              // bleh
              if (route.name === "Home") {
                // reset the stack (b/c streamlist is in the same stack as home)
                navigation.dispatch(
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
              } else if (route.name === "Settings") {
                navigation.dispatch(
                  CommonActions.reset({
                    index: 0,
                    routes: [
                      {
                        name: "Settings",
                      },
                    ],
                  }),
                );
              } else {
                navigation.navigate(route.name as any);
              }
            }}
            style={options.drawerItemStyle}
            tint={options.drawerActiveTintColor as string}
          />
        );
      })}
      {externalItems.map((i, num) => {
        // Handle different label types - string, JSX element, or component
        const renderLabel = () => {
          if (typeof i.label === "string") {
            return i.label;
          }

          // If it's already a JSX element, return it directly
          if (React.isValidElement(i.label)) {
            return i.label;
          }

          // If it's a function (component), call it
          if (typeof i.label === "function") {
            const LabelComponent = i.label;
            return <LabelComponent />;
          }

          // Fallback
          return "Item";
        };

        return (
          <SidebarItem
            key={num}
            icon={i.item}
            label={renderLabel()}
            active={false}
            collapsed={collapsed}
            onPress={() => i.onPress()}
            tint="rgba(189, 110, 134)"
          />
        );
      })}
    </Animated.View>
  );
}
