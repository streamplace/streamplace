import type { NativeStackScreenProps } from "@react-navigation/native-stack";

// iOS-specific: Tab navigator param list
export type IOSTabParamList = {
  HomeTab: undefined;
  GoLiveTab: undefined; // LaunchGoLive screen
  SettingsTab: undefined;
};

// iOS-specific: Root stack includes tabs + other screens
export type IOSRootStackParamList = {
  MainTabs: undefined;
  Stream: { user: string };
  AboutCategory: undefined;
  AccountCategory: undefined;
  StreamingCategory: undefined;
  WebhooksSettings: undefined;
  PrivacyCategory: undefined;
  DanmuCategory: undefined;
  AdvancedCategory: undefined;
  LanguagesCategory: undefined;
  KeyManagement: undefined;
  Login: undefined;
  Multi: { config: string };
  Support: undefined;
  LiveDashboard: undefined;
  AppReturn: { scheme: string };
  About: undefined;
  Download: undefined;
  PopoutChat: { user: string };
  Embed: { user: string };
  InfoWidgetEmbed: undefined;
  DanmuOBS: { user: string };
};

// Flat root stack with all screens (for web/drawer)
export type RootStackParamList = {
  // Home screens
  StreamList: undefined;
  Stream: { user: string };

  // Settings screens
  MainSettings: undefined;
  AboutCategory: undefined;
  AccountCategory: undefined;
  StreamingCategory: undefined;
  WebhooksSettings: undefined;
  PrivacyCategory: undefined;
  DanmuCategory: undefined;
  AdvancedCategory: undefined;
  LanguagesCategory: undefined;
  DeveloperSettings: undefined;
  KeyManagement: undefined;

  // Other screens
  Multi: { config: string };
  Support: undefined;
  MobileGoLive: undefined;
  LiveDashboard: undefined;
  Login: undefined;
  AVSync: undefined;
  AppReturn: { scheme: string };
  About: undefined;
  Download: undefined;
  PopoutChat: { user: string };
  Embed: { user: string };
  InfoWidgetEmbed: undefined;
  LegacyStream: { user: string };
  DanmuOBS: { user: string };
};

// Helper type for screen props
export type RootStackScreenProps<T extends keyof RootStackParamList> =
  NativeStackScreenProps<RootStackParamList, T>;

// Global namespace augmentation for React Navigation
declare global {
  namespace ReactNavigation {
    interface RootParamList extends RootStackParamList {}
  }
}
