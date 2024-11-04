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
  LockKeyhole,
  Menu,
  Settings as SettingsIcon,
} from "@tamagui/lucide-icons";
import { Provider, Settings } from "components";
import Admin from "components/admin";
import StreamList from "components/stream-list/stream-list";
import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import { Pressable } from "react-native";
import { useTheme, View } from "tamagui";
import MultiScreen from "./screens/multi";
import StreamScreen from "./screens/stream";
import SupportScreen from "./screens/support";

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
            path: "stream/:user",
          },
        },
      },
      Multi: "multi/:config",
      Admin: "admin",
      Support: "support",
      Settings: "settings",
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
  const { isWeb } = usePlatform();
  const navigation = useNavigation();
  return (
    <Drawer.Navigator
      initialRouteName="Home"
      screenOptions={{
        headerLeft: () => <NavigationButton />,
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
      {isWeb && (
        <Drawer.Screen
          name="Admin"
          component={Admin}
          options={{
            drawerIcon: () => <LockKeyhole />,
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
          }}
        />
      )}
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
