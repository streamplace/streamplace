import {
  createDrawerNavigator,
  DrawerContentScrollView,
  DrawerItem,
  DrawerItemList,
} from "@react-navigation/drawer";
import {
  CommonActions,
  DrawerActions,
  useLinkTo,
  useNavigation,
  useRoute,
} from "@react-navigation/native";
import { Text, useTheme, useToast, useUrl } from "@streamplace/components";
import AQLink from "components/aqlink";
import Login from "components/login/login";
import LoginModal from "components/login/login-modal";
import Sidebar, { ExternalDrawerItem } from "components/sidebar/sidebar";
import { useBlueskyNotifications } from "hooks/useBlueskyNotifications";
import { useLiveUser } from "hooks/useLiveUser";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  ArrowLeft,
  Book,
  Download,
  ExternalLink,
  Home,
  LogIn,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Settings as SettingsIcon,
  ShieldQuestion,
  User,
  Video,
} from "lucide-react-native";
import React, { Fragment, useEffect, useState } from "react";
import {
  ImageBackground,
  ImageSourcePropType,
  Linking,
  Platform,
  Pressable,
  StatusBar,
  View,
} from "react-native";
import { runOnJS, useAnimatedReaction } from "react-native-reanimated";
import { MainTab, SettingsStack } from "src/navigator";
import AboutScreen from "src/screens/about";
import AppReturnScreen from "src/screens/app-return";
import PopoutChat from "src/screens/chat-popout";
import DanmuOBSScreen from "src/screens/danmu-obs";
import DownloadScreen from "src/screens/download";
import EmbedScreen from "src/screens/embed";
import InfoWidgetEmbed from "src/screens/info-widget-embed";
import LiveDashboard from "src/screens/live-dashboard";
import MobileGoLive from "src/screens/mobile-go-live";
import MultiScreen from "src/screens/multi";
import SupportScreen from "src/screens/support";
import { useStore } from "store";
import {
  useHydrated,
  useNotificationDestination,
  useNotificationToken,
  useUserProfile,
} from "store/hooks";

const Drawer = createDrawerNavigator();

const useExternalItems = (): ExternalDrawerItem[] => {
  const streamplaceUrl = useUrl();
  const { theme } = useTheme();
  return [
    {
      item: React.memo(() => <Book size={24} color={theme.colors.text} />),
      label: (
        <Text variant="h5" style={{ alignSelf: "flex-start" }}>
          Documentation{" "}
          <ExternalLink
            size={16}
            color={theme.colors.mutedForeground}
            style={{
              position: "relative",
              top: 2,
            }}
          />
        </Text>
      ) as any,
      onPress: () => {
        const u = new URL(streamplaceUrl);
        u.pathname = "/docs";
        Linking.openURL(u.toString());
      },
    },
  ];
};

function CustomDrawerContent(props) {
  let { theme } = useTheme();
  return (
    <DrawerContentScrollView {...props}>
      <DrawerItemList {...props} />
      <DrawerItem
        icon={() => <Book size={24} color={theme.colors.text} />}
        label={() => (
          <Text style={{ alignSelf: "flex-start" }}>
            Documentation{" "}
            <ExternalLink
              size={16}
              color="#666"
              style={{
                position: "relative",
                top: 2,
              }}
            />
          </Text>
        )}
        onPress={() => {
          const u = new URL(window.location.href);
          u.pathname = "/docs";
          Linking.openURL(u.toString());
        }}
      />
    </DrawerContentScrollView>
  );
}

const PopupChecker = ({
  setIsLiveDashboard,
}: {
  setIsLiveDashboard: (isLiveDashboard: boolean) => void;
}) => {
  const route = useRoute();
  useEffect(() => {
    if (route.name === "LiveDashboard") {
      setIsLiveDashboard(true);
    } else {
      setIsLiveDashboard(false);
    }
  }, [route.name]);
  return <Fragment />;
};

export default function WebShell() {
  const theme = useTheme();
  const { isWeb, isElectron, isNative, isBrowser } = usePlatform();
  const navigation = useNavigation();
  const hydrate = useStore((state) => state.hydrate);
  const initPushNotifications = useStore(
    (state) => state.initPushNotifications,
  );
  const registerNotificationToken = useStore(
    (state) => state.registerNotificationToken,
  );
  const clearNotification = useStore((state) => state.clearNotification);
  const pollMySegments = useStore((state) => state.pollMySegments);
  const showLoginModal = useStore((state) => state.showLoginModal);
  const closeLoginModal = useStore((state) => state.closeLoginModal);
  const [livePopup, setLivePopup] = useState(false);

  const sidebar = useSidebarControl();
  const [drawerWidth, setDrawerWidth] = useState(sidebar.animatedWidth.value);

  useAnimatedReaction(
    () => sidebar.animatedWidth.value,
    (current) => {
      runOnJS(setDrawerWidth)(current);
    },
    [sidebar.animatedWidth],
  );

  const toast = useToast();

  // Top-level stuff to handle push notification registration
  useEffect(() => {
    hydrate();
    initPushNotifications();
  }, []);
  const notificationToken = useNotificationToken();
  const hydrated = useHydrated();
  useEffect(() => {
    if (notificationToken) {
      registerNotificationToken();
    }
  }, [notificationToken]);

  // Stuff to handle incoming push notification routing
  const notificationDestination = useNotificationDestination();
  const linkTo = useLinkTo();

  useEffect(() => {
    if (notificationDestination) {
      linkTo(notificationDestination);
      clearNotification();
    }
  }, [notificationDestination]);

  // Top-level stuff to handle polling for live streamers
  useEffect(() => {
    let handle: NodeJS.Timeout;
    handle = setInterval(() => {
      pollMySegments();
    }, 2500);
    pollMySegments();
    return () => clearInterval(handle);
  }, []);

  const userIsLive = useLiveUser();
  useBlueskyNotifications();

  let foregroundColor = theme.theme.colors.text || "#fff";

  // are we in the live dashboard?
  const [isLiveDashboard, setIsLiveDashboard] = useState(false);
  useEffect(() => {
    if (!isLiveDashboard && userIsLive) {
      toast.show("You are live!", "Do you want to go to your Live Dashboard?", {
        actionLabel: "Go",
        onAction: () => {
          navigation.navigate("LiveDashboard" as never);
          setLivePopup(false);
        },
        onClose: () => setLivePopup(false),
        variant: "error",
        duration: 8,
      });
    }
  }, [userIsLive]);
  const externalItems = useExternalItems();

  if (!hydrated) {
    return <View />;
  }

  return (
    <>
      <StatusBar barStyle="light-content" />
      <Drawer.Navigator
        initialRouteName="Home"
        screenOptions={{
          // for the custom sidebar
          drawerType: sidebar.isActive ? "permanent" : "front",
          swipeEnabled: !sidebar.isActive,
          drawerStyle: {
            zIndex: 128000,
            width: sidebar.isActive ? drawerWidth : undefined,
          },
          // rest
          headerLeft: () => (
            <>
              {/* this is a hack to give the popup the navigator context */}
              <PopupChecker setIsLiveDashboard={setIsLiveDashboard} />
              <NavigationButton />
            </>
          ),
          headerRight: () => <AvatarButton />,
          drawerActiveTintColor: "#a0287c33",
        }}
        drawerContent={
          sidebar.isActive
            ? (props) => (
                <Sidebar
                  {...props}
                  collapsed={sidebar.isCollapsed}
                  hidden={sidebar.isHidden}
                  widthAnim={sidebar.animatedWidth}
                  externalItems={externalItems}
                />
              )
            : CustomDrawerContent
        }
      >
        <Drawer.Screen
          name="Home"
          component={MainTab}
          options={{
            drawerIcon: () => <Home color={foregroundColor} size={24} />,
            drawerLabel: () => <Text variant="h5">Home</Text>,
            headerTitle: isWeb ? "Home" : "Streamplace",
            headerShown: isWeb,
            title: "Streamplace",
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
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
            },
          }}
        />
        <Drawer.Screen
          name="About"
          component={AboutScreen}
          options={{
            drawerLabel: () => <Text variant="h5">What's Streamplace?</Text>,
            drawerIcon: () => (
              <ShieldQuestion color={foregroundColor} size={24} />
            ),
            drawerItemStyle: isNative ? { display: "none" } : undefined,
          }}
        />
        <Drawer.Screen
          name="Download"
          component={DownloadScreen}
          options={{
            drawerLabel: () => <Text variant="h5">Download</Text>,
            drawerIcon: () => <Download color={foregroundColor} size={24} />,
            drawerItemStyle: isBrowser ? undefined : { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Settings"
          component={SettingsStack}
          options={{
            drawerIcon: () => (
              <SettingsIcon color={foregroundColor} size={24} />
            ),
            drawerLabel: () => <Text variant="h5">Settings</Text>,
            headerShown: false,
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
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
            },
          }}
        />

        <Drawer.Screen
          name="Support"
          component={SupportScreen}
          options={{
            drawerLabel: () => <Text variant="h5">Support</Text>,
            drawerItemStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="LiveDashboard"
          component={LiveDashboard}
          options={{
            drawerLabel: () => <Text variant="h5">Live Dashboard</Text>,
            drawerIcon: () => <Video color={foregroundColor} size={24} />,
            drawerItemStyle: isNative ? { display: "none" } : undefined,
          }}
        />
        <Drawer.Screen
          name="AppReturn"
          component={AppReturnScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="Multi"
          component={MultiScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Login"
          component={Login}
          options={{
            drawerIcon: () => <LogIn color={foregroundColor} size={24} />,
            drawerLabel: () => <Text variant="h5">Login</Text>,
          }}
        />
        <Drawer.Screen
          name="PopoutChat"
          component={PopoutChat}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
            drawerStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Embed"
          component={EmbedScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="InfoWidgetEmbed"
          component={InfoWidgetEmbed}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="DanmuOBS"
          component={DanmuOBSScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="MobileGoLive"
          component={MobileGoLive}
          options={{
            headerTitle: "Go Live",
            drawerItemStyle: isNative ? undefined : { display: "none" },
            drawerLabel: () => <Text variant="h5">Go Live</Text>,
            title: "Go live",
            drawerIcon: () => <Video color={foregroundColor} size={24} />,
            headerShown: false,
          }}
        />
      </Drawer.Navigator>
      <LoginModal visible={showLoginModal} onClose={closeLoginModal} />
    </>
  );
}

const NavigationButton = ({ canGoBack }: { canGoBack?: boolean }) => {
  const sidebar = useSidebarControl();
  const navigation = useNavigation();
  const { theme } = useTheme();

  const handlePress = () => {
    if (sidebar?.isActive) {
      sidebar.toggle();
    }
  };

  const handleGoBackPress = () => {
    if (canGoBack) {
      navigation.goBack();
    } else {
      navigation.dispatch(DrawerActions.toggleDrawer());
    }
  };

  return (
    <View
      style={[
        { flexDirection: "row" },
        {
          marginLeft: Platform.OS === "android" ? 0 : 12,
          marginRight: Platform.OS === "android" ? 12 : 0,
        },
      ]}
    >
      {sidebar?.isActive ? (
        <>
          <Pressable style={{ padding: 5 }} onPress={handlePress}>
            {sidebar.isCollapsed ? (
              <PanelLeftOpen size={24} color={theme.colors.accentForeground} />
            ) : (
              <PanelLeftClose size={24} color={theme.colors.accentForeground} />
            )}
          </Pressable>
          {canGoBack && (
            <Pressable
              style={{ marginLeft: 10, paddingVertical: 5 }}
              onPress={handleGoBackPress}
            >
              <ArrowLeft size={24} color={theme.colors.accentForeground} />
            </Pressable>
          )}
        </>
      ) : (
        <Pressable style={{ padding: 5 }} onPress={handleGoBackPress}>
          {canGoBack ? (
            <ArrowLeft size={24} color={theme.colors.accentForeground} />
          ) : (
            <Menu size={24} color={theme.colors.accentForeground} />
          )}
        </Pressable>
      )}
    </View>
  );
};

const AvatarButton = () => {
  const userProfile = useUserProfile();
  let source: ImageSourcePropType | undefined = undefined;
  let opacity = 1;
  const targetScreen: any = userProfile
    ? { screen: "Settings", params: { screen: "AccountCategory" } }
    : { screen: "Login", params: {} };

  if (userProfile) {
    source = { uri: userProfile.avatar };
    opacity = 0;
  }
  return (
    <AQLink to={targetScreen}>
      <ImageBackground
        // defeat cursed-ass caching on ios; image sticks around when source is undefined
        key={source?.uri ?? "default"}
        source={source}
        style={{
          width: 40,
          height: 40,
          borderRadius: 24,
          overflow: "hidden",
          marginRight: 10,
          backgroundColor: "black",
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <User size={24} color="white" style={{ zIndex: -2 }} />
      </ImageBackground>
    </AQLink>
  );
};
