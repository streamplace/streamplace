import "@expo/metro-runtime";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { Provider } from "components";
import StreamList from "components/stream-list/stream-list";
import { Appearance, SafeAreaView } from "react-native";
import { useTheme } from "tamagui";
import StreamScreen from "./screens/stream";
import { NavigationContainer } from "@react-navigation/native";

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
    },
  },
};

export default function Router() {
  return (
    <Provider linking={linking}>
      <RenderArea />
    </Provider>
  );
}

const RenderArea = () => {
  const theme = useTheme();
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.background.val }}>
      <Stack.Navigator>
        <Stack.Screen
          name="Home"
          component={HomeScreen}
          options={{ headerShown: true, headerTitle: "Aquareum" }}
        />
        <Stack.Screen
          name="Stream"
          component={StreamScreen}
          options={{
            headerShown: true,
            headerTitle: "Stream",
          }}
        />
      </Stack.Navigator>
    </SafeAreaView>
  );
};
