import "@expo/metro-runtime";
import { createDrawerNavigator } from "@react-navigation/drawer";
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
  Home,
  LogIn,
  Menu,
  Settings as SettingsIcon,
  User,
  ShieldQuestion,
  Download,
  X,
  Video,
} from "@tamagui/lucide-icons";
import { Provider, Settings } from "components";
import AQLink from "components/aqlink";
import Login from "components/login/login";
import StreamList from "components/stream-list/stream-list";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import usePlatform from "hooks/usePlatform";
import { useEffect, useState } from "react";
import {
  ImageBackground,
  ImageSourcePropType,
  Pressable,
  StatusBar,
} from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { useTheme, Text, View, H3, Button } from "tamagui";
import AppReturnScreen from "./screens/app-return";
import MultiScreen from "./screens/multi";
import StreamScreen from "./screens/stream";
import SupportScreen from "./screens/support";
import AboutScreen from "./screens/about";
import DownloadScreen from "./screens/download";
import { hydrate, selectHydrated } from "features/base/baseSlice";
import AVSyncScreen from "./screens/av-sync";
import {
  clearNotification,
  initPushNotifications,
  registerNotificationToken,
  selectNotificationDestination,
  selectNotificationToken,
} from "features/platform/platformSlice.native";
import { pollSegments } from "features/streamplace/streamplaceSlice";
import { useLiveUser } from "hooks/useLiveUser";
import { useToastController } from "@tamagui/toast";
import LiveDashboard from "./screens/live-dashboard";
function HomeScreen() {
  return (
    <View f={1}>
      <StreamList contentContainerStyle={{ paddingTop: "$3" }}></StreamList>
    </View>
  );
}
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
  AVSync: undefined;
  AppReturn: { scheme: string };
  About: undefined;
  Download: undefined;
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
      AVSync: "sync-test",
      AppReturn: "app-return/:scheme",
      About: "about",
      Download: "download",
    },
  },
};

const Drawer = createDrawerNavigator();

const NavigationButton = ({ canGoBack }: { canGoBack?: boolean }) => {
  const navigation = useNavigation();
  return (
    <Pressable
      style={{ padding: 10 }}
      onPress={() => {
        if (canGoBack) {
          navigation.goBack();
        } else {
          navigation.dispatch(DrawerActions.toggleDrawer());
        }
      }}
    >
      {canGoBack ? <ArrowLeft /> : <Menu />}
    </Pressable>
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
    dispatch(pollSegments());
    const interval = setInterval(() => {
      dispatch(pollSegments());
    }, 5000);
    return () => clearInterval(interval);
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
          headerLeft: () => <NavigationButton />,
          headerRight: () => <AvatarButton />,
          drawerActiveTintColor: theme.accentColor.val,
          unmountOnBlur: true,
        }}
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
      </Drawer.Navigator>
      {livePopup && (
        <View
          position="absolute"
          bottom="$8"
          f={1}
          alignItems="center"
          width="100%"
        >
          <View
            backgroundColor="#cc0000"
            f={1}
            alignItems="center"
            padding="$4"
            borderRadius="$4"
            cursor="pointer"
            onPress={() => {
              navigation.navigate("LiveDashboard");
              setLivePopup(false);
            }}
            position="relative"
          >
            <H3>✨YOU ARE LIVE!!!✨</H3>
            <Button
              position="absolute"
              top="$0"
              right="$0"
              onPress={(e) => {
                e.stopPropagation();
                setLivePopup(false);
              }}
              marginRight={-15}
              marginTop={-5}
              backgroundColor="transparent"
            >
              <X />
            </Button>
            <Text>
              {isNative ? "Tap" : "Click"} here to go to the live dashboard
            </Text>
          </View>
        </View>
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
