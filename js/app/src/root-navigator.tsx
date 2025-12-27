import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { Platform } from "react-native";
import PopoutChat from "src/screens/chat-popout";
import DanmuOBSScreen from "src/screens/danmu-obs";
import EmbedScreen from "src/screens/embed";
import InfoWidgetEmbed from "src/screens/info-widget-embed";
import MobileStream from "src/screens/mobile-stream";
import type { RootStackParamList } from "./navigation-types";

const Stack = createNativeStackNavigator<RootStackParamList>();

// The root navigator, containing full-screen items that we want to hide the tab bar on
// On mobile, we'll want these to contain a back button to redirect to somewhere that
// has tabs (e.g. Home)
export const RootNavigator = () => {
  return (
    <Stack.Navigator
      initialRouteName="Stream"
      screenOptions={{
        headerShown: Platform.OS !== "web",
      }}
    >
      <Stack.Screen name="Stream" component={MobileStream} />
      <Stack.Screen name="PopoutChat" component={PopoutChat} />
      <Stack.Screen name="Embed" component={EmbedScreen} />
      <Stack.Screen name="InfoWidgetEmbed" component={InfoWidgetEmbed} />
      <Stack.Screen name="DanmuOBS" component={DanmuOBSScreen} />
    </Stack.Navigator>
  );
};

// TypeScript helpers
export type RootNavigatorType = typeof Stack;

declare module "@react-navigation/core" {
  interface RootNavigator extends RootNavigatorType {}
}
