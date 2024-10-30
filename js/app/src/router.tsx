import "@expo/metro-runtime";
import { createDrawerNavigator } from "@react-navigation/drawer";
import { DrawerActions, useNavigation } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { Home, Menu, Settings as SettingsIcon } from "@tamagui/lucide-icons";
import { Provider, Settings } from "components";
import StreamList from "components/stream-list/stream-list";
import { Pressable, SafeAreaView } from "react-native";
import { useTheme } from "tamagui";
import MultiScreen from "./screens/multi";
import StreamScreen from "./screens/stream";

function HomeScreen() {
  return <StreamList></StreamList>;
}

const Stack = createNativeStackNavigator();

const linking = {
  prefixes: ["http://localhost:38081", "tv.aquareum://", "tv.aquareum.dev://"],
  config: {
    screens: {
      Home: "",
      Stream: "stream/:user",
      Multi: "multi/:config",
    },
  },
};

const Drawer = createDrawerNavigator();

export default function Router() {
  return (
    <Provider linking={linking}>
      <AquareumDrawer />
    </Provider>
  );
}

export function AquareumDrawer() {
  const theme = useTheme();
  return (
    <Drawer.Navigator
      initialRouteName="Home"
      screenOptions={{
        headerLeft: ({}) => {
          const navigation = useNavigation();
          return (
            <Pressable
              style={{ padding: 10 }}
              onPress={() => navigation.dispatch(DrawerActions.toggleDrawer())}
            >
              <Menu />
            </Pressable>
          );
        },
        drawerActiveTintColor: theme.accentColor.val,
      }}
    >
      <Drawer.Screen
        name="Aquareum"
        component={MainTab}
        options={{ drawerIcon: () => <Home /> }}
      />
      <Drawer.Screen
        name="Settings"
        component={Settings}
        options={{ drawerIcon: () => <SettingsIcon /> }}
      />
    </Drawer.Navigator>
  );
}

const MainTab = () => {
  const theme = useTheme();
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.background.val }}>
      <Stack.Navigator screenOptions={{ headerShown: false }}>
        <Stack.Screen
          name="Home"
          component={HomeScreen}
          options={{ headerTitle: "Aquareum" }}
        />
        <Stack.Screen
          name="Stream"
          component={StreamScreen}
          options={{
            headerTitle: "Stream",
          }}
        />
        <Stack.Screen
          name="Multi"
          component={MultiScreen}
          options={{ headerTitle: "Multi" }}
        />
      </Stack.Navigator>
    </SafeAreaView>
  );
};
