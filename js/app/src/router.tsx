import "@expo/metro-runtime";
import { createDrawerNavigator } from "@react-navigation/drawer";
import {
  CommonActions,
  DrawerActions,
  LinkingOptions,
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
  Video,
} from "@tamagui/lucide-icons";
import { Provider, Settings } from "components";
import AQLink from "components/aqlink";
import Login from "components/login/login";
import StreamList from "components/stream-list/stream-list";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import {
  ImageBackground,
  ImageSourcePropType,
  Pressable,
  StatusBar,
} from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Text, useTheme, View } from "tamagui";
import AppReturnScreen from "./screens/app-return";
import LiveScreen from "./screens/live";
import MultiScreen from "./screens/multi";
import StreamScreen from "./screens/stream";
import SupportScreen from "./screens/support";
import WebcamScreen from "./screens/webcam";
import StreamKeyScreen from "./screens/stream-key";
import { hydrate, selectHydrated } from "features/base/baseSlice";
function HomeScreen() {
  return (
    <View f={1}>
      <StreamList contentContainerStyle={{ paddingTop: "$3" }}></StreamList>
    </View>
  );
}
const Stack = createNativeStackNavigator();

const linking: LinkingOptions<ReactNavigation.RootParamList> = {
  prefixes: ["tv.aquareum://", "tv.aquareum.dev://"],
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
      Live: "live",
      Webcam: "live/webcam",
      StreamKey: "live/stream-key",
      Login: "login",
      AppReturn: "app-return/:scheme",
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
  const { initPushNotifications, isWeb, isElectron } = usePlatform();
  useEffect(() => {
    initPushNotifications();
  }, []);
  if (isWeb && !isElectron) {
    linking.prefixes.push(document.location.origin);
  }
  return (
    <Provider linking={linking}>
      <AquareumDrawer />
    </Provider>
  );
}

export function AquareumDrawer() {
  const theme = useTheme();
  const { isWeb, isElectron } = usePlatform();
  const navigation = useNavigation();
  const dispatch = useAppDispatch();
  useEffect(() => {
    dispatch(hydrate());
    // const params = new URLSearchParams(document.location.search);
    // if (params.has("code")) {
    //   navigation.dispatch(
    //     CommonActions.reset({
    //       index: 0,
    //       routes: [{ name: "Login" }],
    //     }),
    //   );
    // }
  }, []);
  const hydrated = useAppSelector(selectHydrated);
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
            headerTitle: "Aquareum",
            headerShown: isWeb,
            title: "Aquareum",
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
          name="Settings"
          component={Settings}
          options={{
            drawerIcon: () => <SettingsIcon />,
            drawerLabel: () => <Text>Settings</Text>,
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
          name="Support"
          component={SupportScreen}
          options={{
            drawerLabel: () => <Text>Support</Text>,
            drawerItemStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Live"
          component={LiveScreen}
          options={{
            drawerLabel: () => <Text>Go Live</Text>,
            drawerIcon: () => <Video />,
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
          name="Webcam"
          component={WebcamScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="StreamKey"
          component={StreamKeyScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
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
        options={{ headerTitle: "Aquareum", title: "Aquareum" }}
      />
      <Stack.Screen
        name="Stream"
        component={StreamScreen}
        options={{
          headerTitle: "Stream",
          title: "Aquareum Stream",
        }}
      />
    </Stack.Navigator>
  );
};
