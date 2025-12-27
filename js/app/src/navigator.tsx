import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { Settings } from "components";
import { AboutCategorySettings } from "components/settings/about-category-settings";
import { AccountCategorySettings } from "components/settings/account-category-settings";
import { AdvancedCategorySettings } from "components/settings/advanced-category-settings";
import { DanmuCategorySettings } from "components/settings/danmu-category-settings";
import KeyManager from "components/settings/key-manager";
import { LanguagesCategorySettings } from "components/settings/languages-category-settings";
import { PrivacyCategorySettings } from "components/settings/privacy-category-settings";
import { StreamingCategorySettings } from "components/settings/streaming-category-settings";
import WebhookManager from "components/settings/webhook-manager";
import HomeScreen from "./screens/home";
import MobileStream from "./screens/mobile-stream";

const Stack = createNativeStackNavigator();

export const MainTab = () => {
  return (
    <Stack.Navigator
      initialRouteName="StreamList"
      screenOptions={{
        headerShown: false,
      }}
    >
      <Stack.Screen
        name="StreamList"
        component={HomeScreen}
        options={{ headerTitle: "Streamplace", title: "Streamplace" }}
      />
      <Stack.Screen
        name="Stream"
        component={MobileStream}
        options={{
          headerTitle: "Stream",
          title: "Streamplace Stream",
          headerShown: false,
        }}
      />
    </Stack.Navigator>
  );
};

export const SettingsStack = () => {
  return (
    <Stack.Navigator
      initialRouteName="MainSettings"
      screenOptions={{
        headerShown: false,
      }}
    >
      <Stack.Screen
        name="MainSettings"
        component={Settings}
        options={{ headerTitle: "Settings", title: "Settings" }}
      />
      <Stack.Screen
        name="AboutCategory"
        component={AboutCategorySettings}
        options={{ headerTitle: "About", title: "About" }}
      />
      <Stack.Screen
        name="AccountCategory"
        component={AccountCategorySettings}
        options={{ headerTitle: "Account", title: "Account" }}
      />
      <Stack.Screen
        name="StreamingCategory"
        component={StreamingCategorySettings}
        options={{ headerTitle: "Streaming", title: "Streaming" }}
      />
      <Stack.Screen
        name="WebhooksSettings"
        component={WebhookManager}
        options={{ headerTitle: "Webhooks", title: "Webhooks" }}
      />
      <Stack.Screen
        name="PrivacyCategory"
        component={PrivacyCategorySettings}
        options={{
          headerTitle: "Privacy & Security",
          title: "Privacy & Security",
        }}
      />
      <Stack.Screen
        name="DanmuCategory"
        component={DanmuCategorySettings}
        options={{ headerTitle: "Danmu", title: "Danmu" }}
      />
      <Stack.Screen
        name="AdvancedCategory"
        component={AdvancedCategorySettings}
        options={{ headerTitle: "Advanced", title: "Advanced" }}
      />
      <Stack.Screen
        name="LanguagesCategory"
        component={LanguagesCategorySettings}
        options={{ headerTitle: "Languages", title: "Languages" }}
      />
      <Stack.Screen
        name="KeyManagement"
        component={KeyManager}
        options={{ headerTitle: "Key Manager", title: "Key Manager" }}
      />
    </Stack.Navigator>
  );
};
