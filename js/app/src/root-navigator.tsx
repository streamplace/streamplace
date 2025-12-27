import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { Settings } from "components";
import Login from "components/login/login";
import { AboutCategorySettings } from "components/settings/about-category-settings";
import { AccountCategorySettings } from "components/settings/account-category-settings";
import { AdvancedCategorySettings } from "components/settings/advanced-category-settings";
import { DanmuCategorySettings } from "components/settings/danmu-category-settings";
import KeyManager from "components/settings/key-manager";
import { LanguagesCategorySettings } from "components/settings/languages-category-settings";
import { PrivacyCategorySettings } from "components/settings/privacy-category-settings";
import { StreamingCategorySettings } from "components/settings/streaming-category-settings";
import WebhookManager from "components/settings/webhook-manager";
import AboutScreen from "src/screens/about";
import AppReturnScreen from "src/screens/app-return";
import PopoutChat from "src/screens/chat-popout";
import DanmuOBSScreen from "src/screens/danmu-obs";
import DownloadScreen from "src/screens/download";
import EmbedScreen from "src/screens/embed";
import HomeScreen from "src/screens/home";
import InfoWidgetEmbed from "src/screens/info-widget-embed";
import LiveDashboard from "src/screens/live-dashboard";
import MobileGoLive from "src/screens/mobile-go-live";
import MobileStream from "src/screens/mobile-stream";
import MultiScreen from "src/screens/multi";
import SupportScreen from "src/screens/support";
import type { RootStackParamList } from "./navigation-types";

const Stack = createNativeStackNavigator<RootStackParamList>();

export const RootNavigator = () => {
  return (
    <Stack.Navigator
      initialRouteName="StreamList"
      screenOptions={{
        headerShown: false,
      }}
    >
      {/* Home screens */}
      <Stack.Screen name="StreamList" component={HomeScreen} />
      <Stack.Screen name="Stream" component={MobileStream} />

      {/* Settings screens */}
      <Stack.Screen name="MainSettings" component={Settings} />
      <Stack.Screen name="AboutCategory" component={AboutCategorySettings} />
      <Stack.Screen
        name="AccountCategory"
        component={AccountCategorySettings}
      />
      <Stack.Screen
        name="StreamingCategory"
        component={StreamingCategorySettings}
      />
      <Stack.Screen name="WebhooksSettings" component={WebhookManager} />
      <Stack.Screen
        name="PrivacyCategory"
        component={PrivacyCategorySettings}
      />
      <Stack.Screen name="DanmuCategory" component={DanmuCategorySettings} />
      <Stack.Screen
        name="AdvancedCategory"
        component={AdvancedCategorySettings}
      />
      <Stack.Screen
        name="LanguagesCategory"
        component={LanguagesCategorySettings}
      />
      <Stack.Screen name="KeyManagement" component={KeyManager} />

      {/* Other screens */}
      <Stack.Screen name="MobileGoLive" component={MobileGoLive} />
      <Stack.Screen name="Login" component={Login} />
      <Stack.Screen name="Multi" component={MultiScreen} />
      <Stack.Screen name="Support" component={SupportScreen} />
      <Stack.Screen name="LiveDashboard" component={LiveDashboard} />
      <Stack.Screen name="AppReturn" component={AppReturnScreen} />
      <Stack.Screen name="About" component={AboutScreen} />
      <Stack.Screen name="Download" component={DownloadScreen} />
      <Stack.Screen name="PopoutChat" component={PopoutChat} />
      <Stack.Screen name="Embed" component={EmbedScreen} />
      <Stack.Screen name="InfoWidgetEmbed" component={InfoWidgetEmbed} />
      <Stack.Screen name="DanmuOBS" component={DanmuOBSScreen} />
    </Stack.Navigator>
  );
};

// Export the type for module augmentation
export type RootNavigatorType = typeof Stack;

// Augment @react-navigation/core with our root navigator type
declare module "@react-navigation/core" {
  interface RootNavigator extends RootNavigatorType {}
}
