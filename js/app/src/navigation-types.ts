import type { NavigatorScreenParams } from "@react-navigation/native";
import type { NativeStackScreenProps } from "@react-navigation/native-stack";

export type SettingsStackParamList = {
  MainSettings: undefined;
  AboutCategory: undefined;
  AccountCategory: undefined;
  StreamingCategory: undefined;
  WebhooksSettings: undefined;
  BackupSettings: undefined;
  PrivacyCategory: undefined;
  NotificationsCategory: undefined;
  DanmuCategory: undefined;
  AdvancedCategory: undefined;
  LanguagesCategory: undefined;
  MultistreamCategory: undefined;
  RecommendationsSettings: undefined;
  KeyManagement: undefined;
  BadgeSelection: undefined;
  BadgeIssuer: undefined;
  BrandingAdmin: undefined;
};

export type HomeStackParamList = {
  HomeMain: undefined;
  About: undefined;
  Download: undefined;
  LiveDashboard: undefined;
  Login: undefined;
  Multi: { config: string };
  Support: undefined;
  Upload: undefined;
  UploadVideo: { tid: string };
  UploadDrafts: undefined;
  UploadLivestreams: undefined;
  UploadVideos: undefined;
};

// Videos tab navigator
export type VideosStackParamList = {
  VideoList: undefined;
  UserVideoList: { did: string };
};

// Main tab navigator
export type TabParamList = {
  HomeTab: NavigatorScreenParams<HomeStackParamList>;
  VideosTab: NavigatorScreenParams<VideosStackParamList>;
  GoLiveTab: undefined;
  SettingsTab: NavigatorScreenParams<SettingsStackParamList>;
};

// Root stack navigator
export type RootStackParamList = {
  MainTabs: NavigatorScreenParams<TabParamList>;

  Stream: { user: string };
  MobileGoLive: undefined;

  AppReturn: { scheme: string };
  PopoutChat: { user: string };
  Embed: { user: string };
  InfoWidgetEmbed: undefined;
  DanmuOBS: { user: string };
  PopoutStreamMonitor: undefined;
  PopoutInfoWidget: undefined;
  PopoutMultistream: undefined;
  PopoutLivestream: undefined;
  Video: undefined;
  Vod: { user: string; tid: string };
  VodEmbed: { user: string; tid: string };
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
