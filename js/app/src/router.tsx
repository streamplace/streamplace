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
  Video,
  User,
} from "@tamagui/lucide-icons";
import { Provider, Settings } from "components";
import StreamList from "components/stream-list/stream-list";
import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import { ImageBackground, ImageSourcePropType, Pressable } from "react-native";
import { useTheme, View } from "tamagui";
import MultiScreen from "./screens/multi";
import StreamScreen from "./screens/stream";
import SupportScreen from "./screens/support";
import GoLiveScreen from "./screens/golive";
import Login from "components/login/login";
import { selectUserProfile } from "features/bluesky/blueskySlice";
import { useAppSelector } from "store/hooks";
import AQLink from "components/aqlink";

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
      Login: "login",
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
  const navigation = useNavigation();
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
  useEffect(() => {
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
  return (
    <Drawer.Navigator
      initialRouteName="Home"
      screenOptions={{
        headerLeft: () => <NavigationButton />,
        headerRight: () => <AvatarButton />,
        drawerActiveTintColor: theme.accentColor.val,
        headerStyle: {},
      }}
    >
      <Drawer.Screen
        name="Home"
        component={MainTab}
        options={{
          drawerIcon: () => <Home />,
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
        options={{ drawerIcon: () => <SettingsIcon /> }}
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
          drawerLabel: () => null,
          drawerItemStyle: { display: "none" },
        }}
      />
      <Drawer.Screen
        name="Login"
        component={Login}
        options={{ drawerIcon: () => <LogIn /> }}
      />
      {isElectron && (
        <Drawer.Screen
          name="GoLive"
          component={GoLiveScreen}
          options={{ headerTitle: "Go Live", drawerIcon: () => <Video /> }}
        />
      )}
    </Drawer.Navigator>
  );
}

const MainTab = () => {
  const theme = useTheme();
  const { isWeb } = usePlatform();
  return (
    // <SafeAreaView style={{ flex: 1, backgroundColor: theme.background.val }}>
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
    // </SafeAreaView>
  );
};
