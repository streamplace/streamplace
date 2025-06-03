import "@expo/metro-runtime";
import {
  createDrawerNavigator,
  DrawerContentScrollView,
  DrawerItem,
  DrawerItemList,
} from "@react-navigation/drawer";
import {
  CommonActions,
  DrawerActions,
  LinkingOptions,
  NavigatorScreenParams,
  useLinkTo,
  useNavigation,
} from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import {
  ArrowLeft,
  Book,
  Download,
  ExternalLink,
  Home,
  LogIn,
  Menu,
  Notebook,
  PanelLeftClose,
  PanelLeftOpen,
  Settings as SettingsIcon,
  ShieldQuestion,
  User,
  Video,
} from "@tamagui/lucide-icons";
import { useToastController } from "@tamagui/toast";
import { Provider, Settings } from "components";
import AQLink from "components/aqlink";
import Login from "components/login/login";
import Popup from "components/popup";
import Sidebar, { ExternalDrawerItem } from "components/sidebar/sidebar";
import { hydrate, selectHydrated } from "features/base/baseSlice";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import {
  clearNotification,
  initPushNotifications,
  registerNotificationToken,
  selectNotificationDestination,
  selectNotificationToken,
} from "features/platform/platformSlice.native";
import {
  pollMySegments,
  pollSegments,
} from "features/streamplace/streamplaceSlice";
import { useLiveUser } from "hooks/useLiveUser";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import { ReactElement, useEffect, useState } from "react";
import {
  ImageBackground,
  ImageSourcePropType,
  Linking,
  Platform,
  Pressable,
  StatusBar,
} from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { H3, Text, useTheme, View } from "tamagui";
import AboutScreen from "./screens/about";
import AppReturnScreen from "./screens/app-return";
import AVSyncScreen from "./screens/av-sync";
import PopoutChat from "./screens/chat-popout";
import DownloadScreen from "./screens/download";
import EmbedScreen from "./screens/embed";
import LiveDashboard from "./screens/live-dashboard";
import MultiScreen from "./screens/multi";
import StreamScreen from "./screens/stream";
import SupportScreen from "./screens/support";

// probabl should move this
import SignUp from "components/login/signup";
import { loadStateFromStorage } from "features/base/sidebarSlice";
import { store } from "store/store";
import HomeScreen from "./screens/home";

import {
  configureReanimatedLogger,
  ReanimatedLogLevel,
} from "react-native-reanimated";

// slows down the whole app
configureReanimatedLogger({
  level: ReanimatedLogLevel.warn,
  strict: false,
});

store.dispatch(loadStateFromStorage());

const Stack = createNativeStackNavigator();

type HomeStackParamList = {
  StreamList: undefined;
  Stream: { user: string };
};

type RootStackParamList = {
  Home: NavigatorScreenParams<HomeStackParamList>;
  Multi: { config: string };
  Support: undefined;
  Settings: undefined;
  GoLive: undefined;
  LiveDashboard: undefined;
  Login: undefined;
  Signup: undefined;
  AVSync: undefined;
  AppReturn: { scheme: string };
  About: undefined;
  Download: undefined;
  PopoutChat: { user: string };
  Embed: { user: string };
};

declare global {
  namespace ReactNavigation {
    interface RootParamList extends RootStackParamList {}
  }
}

const linking: LinkingOptions<ReactNavigation.RootParamList> = {
  prefixes: ["place.stream://", "place.stream.dev://"],
  config: {
    screens: {
      Home: {
        screens: {
          StreamList: "",
          Stream: {
            path: ":user",
          },
        },
      },
      Multi: "multi/:config",
      Support: "support",
      Settings: "settings",
      GoLive: "golive",
      LiveDashboard: "live",
      Login: "login",
      Signup: "signup",
      AVSync: "sync-test",
      AppReturn: "app-return/:scheme",
      About: "about",
      Download: "download",
      PopoutChat: "chat-popout/:user",
      Embed: "embed/:user",
    },
  },
};

const Drawer = createDrawerNavigator();

const NavigationButton = ({ canGoBack }: { canGoBack?: boolean }) => {
  const sidebar = useSidebarControl();
  const navigation = useNavigation();

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

  let icon: ReactElement | null = null;
  if (sidebar?.isActive) {
    if (sidebar.isCollapsed) {
      icon = <PanelLeftOpen />;
    } else {
      icon = <PanelLeftClose />;
    }
  }

  return (
    <View
      flexDirection="row"
      marginLeft={Platform.OS === "android" ? "$0" : "$3"}
      marginRight={Platform.OS === "android" ? "$3" : "$0"}
    >
      {icon && (
        <Pressable style={{ padding: 5 }} onPress={handlePress}>
          {icon}
        </Pressable>
      )}
      <Pressable style={{ padding: 5 }} onPress={handleGoBackPress}>
        {canGoBack ? <ArrowLeft /> : sidebar?.isActive || <Menu />}
      </Pressable>
    </View>
  );
};

const AvatarButton = () => {
  const userProfile = useAppSelector(selectUserProfile);
  let source: ImageSourcePropType | undefined = undefined;
  let opacity = 1;
  if (userProfile) {
    source = { uri: userProfile.avatar };
    opacity = 0;
  }
  return (
    <AQLink to={{ screen: "Login", params: {} }}>
      <ImageBackground
        // defeat cursed-ass caching on ios; image sticks around when source is undefined
        key={source?.uri ?? "default"}
        source={source}
        style={{
          width: 40,
          height: 40,
          borderRadius: 20,
          overflow: "hidden",
          marginRight: 10,
          backgroundColor: "black",
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <User opacity={opacity}></User>
      </ImageBackground>
    </AQLink>
  );
};

const EXTERNAL_ITEMS: ExternalDrawerItem[] = [
  {
    item: Book as any,
    label: (
      <Text alignSelf="flex-start">
        Documentation{" "}
        <ExternalLink size={16} paddingLeft={4} position="relative" top={2} />
      </Text>
    ) as any,
    onPress: () => {
      const u = new URL(window.location.href);
      u.pathname = "/docs";
      Linking.openURL(u.toString());
    },
  },
];

// TODO: merge in ^
function CustomDrawerContent(props) {
  return (
    <DrawerContentScrollView {...props}>
      <DrawerItemList {...props} />
      <DrawerItem
        icon={() => <Book />}
        label={() => (
          <Text alignSelf="flex-start">
            Documentation{" "}
            <ExternalLink size={16} pl={4} position="relative" top={2} />
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

export default function Router() {
  const { isWeb, isElectron } = usePlatform();
  useEffect(() => {
    if (isWeb && !isElectron) {
      linking.prefixes.push(document.location.origin);
    }
  }, []);
  return (
    <Provider linking={linking}>
      <StreamplaceDrawer />
    </Provider>
  );
}

export function StreamplaceDrawer() {
  const theme = useTheme();
  const { isWeb, isElectron, isNative, isBrowser } = usePlatform();
  const navigation = useNavigation();
  const dispatch = useAppDispatch();
  const [poppedUp, setPoppedUp] = useState(false);
  const [livePopup, setLivePopup] = useState(false);

  const sidebar = useSidebarControl();

  // Top-level stuff to handle push notification registration
  useEffect(() => {
    dispatch(hydrate());
    dispatch(initPushNotifications());
  }, []);
  const notificationToken = useAppSelector(selectNotificationToken);
  const userProfile = useAppSelector(selectUserProfile);
  const hydrated = useAppSelector(selectHydrated);
  useEffect(() => {
    if (notificationToken) {
      dispatch(registerNotificationToken());
    }
  }, [notificationToken, userProfile]);

  // Stuff to handle incoming push notification routing
  const notificationDestination = useAppSelector(selectNotificationDestination);
  const linkTo = useLinkTo();

  useEffect(() => {
    if (notificationDestination) {
      linkTo(notificationDestination);
      dispatch(clearNotification());
    }
  }, [notificationDestination]);

  // Top-level stuff to handle polling for live streamers
  useEffect(() => {
    let handle: NodeJS.Timeout;
    const doSegments = () => {
      handle = setTimeout(doMySegments, 2500);
      dispatch(pollSegments());
    };
    const doMySegments = () => {
      handle = setTimeout(doSegments, 2500);
      dispatch(pollMySegments());
    };
    doSegments();
    return () => clearTimeout(handle);
  }, []);

  const userIsLive = useLiveUser();
  const toast = useToastController();

  useEffect(() => {
    if (userIsLive && !poppedUp) {
      setPoppedUp(true);
      setLivePopup(true);
    }
  }, [userIsLive, poppedUp]);

  if (!hydrated) {
    return <View />;
  }
  return (
    <>
      <StatusBar backgroundColor={theme.background.val} />
      <Drawer.Navigator
        initialRouteName="Home"
        screenOptions={{
          // for the custom sidebar
          drawerType: sidebar.isActive ? "permanent" : "front",
          swipeEnabled: !sidebar.isActive,
          drawerStyle: {
            // afaict the drawer is a RN Animated component internally
            width: sidebar.isActive
              ? (sidebar.animatedWidth as any)
              : undefined,
          },
          // rest
          headerLeft: () => <NavigationButton />,
          headerRight: () => <AvatarButton />,
          drawerActiveTintColor: theme.accentColor.val,
          unmountOnBlur: true,
        }}
        drawerContent={
          sidebar.isActive
            ? (props) => (
                <Sidebar
                  {...props}
                  collapsed={sidebar.isCollapsed}
                  hidden={sidebar.isHidden}
                  widthAnim={sidebar.animatedWidth}
                  externalItems={EXTERNAL_ITEMS}
                />
              )
            : CustomDrawerContent
        }
      >
        <Drawer.Screen
          name="Home"
          component={MainTab}
          options={{
            drawerIcon: () => <Home />,
            drawerLabel: () => <Text>Home</Text>,
            headerTitle: "Streamplace",
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
            drawerLabel: () => <Text>What's Streamplace?</Text>,
            drawerIcon: () => <ShieldQuestion />,
            drawerItemStyle: isNative ? { display: "none" } : undefined,
          }}
        />
        <Drawer.Screen
          name="Download"
          component={DownloadScreen}
          options={{
            drawerLabel: () => <Text>Download</Text>,
            drawerIcon: () => <Download />,
            drawerItemStyle: isBrowser ? undefined : { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Settings"
          component={Settings}
          options={{
            drawerIcon: () => <SettingsIcon />,
            drawerLabel: () => <Text>Settings</Text>,
          }}
        />

        <Drawer.Screen
          name="Support"
          component={SupportScreen}
          options={{
            drawerLabel: () => <Text>Support</Text>,
            drawerItemStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="LiveDashboard"
          component={LiveDashboard}
          options={{
            drawerLabel: () => <Text>Live Dashboard</Text>,
            drawerIcon: () => <Video />,
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
          name="AVSync"
          component={AVSyncScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="Login"
          component={Login}
          options={{
            drawerIcon: () => <LogIn />,
            drawerLabel: () => <Text>Login</Text>,
          }}
        />
        <Drawer.Screen
          name="Signup"
          component={SignUp}
          options={{
            drawerIcon: () => <Notebook />,
            drawerItemStyle: { display: "none" },
            drawerLabel: () => <Text>Sign up</Text>,
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
      </Drawer.Navigator>
      {livePopup && (
        <Popup
          onPress={() => {
            navigation.navigate("LiveDashboard");
            setLivePopup(false);
          }}
          onClose={() => {
            setLivePopup(false);
          }}
          containerProps={{
            bottom: "$8",
          }}
          bubbleProps={{
            cursor: "pointer",
            backgroundColor: "#cc0000",
          }}
        >
          <H3 textAlign="center">✨YOU ARE LIVE!!!✨</H3>
          <Text>
            {isNative ? "Tap" : "Click"} here to go to the live dashboard
          </Text>
        </Popup>
      )}
    </>
  );
}

const MainTab = () => {
  const theme = useTheme();
  const { isWeb } = usePlatform();
  return (
    <Stack.Navigator
      initialRouteName="StreamList"
      screenOptions={{
        headerLeft: ({ canGoBack }) => (
          <NavigationButton canGoBack={canGoBack} />
        ),
        headerRight: () => <AvatarButton />,
        headerShown: !isWeb,
      }}
    >
      <Stack.Screen
        name="StreamList"
        component={HomeScreen}
        options={{ headerTitle: "Streamplace", title: "Streamplace" }}
      />
      <Stack.Screen
        name="Stream"
        component={StreamScreen}
        options={{
          headerTitle: "Stream",
          title: "Streamplace Stream",
        }}
      />
    </Stack.Navigator>
  );
};
