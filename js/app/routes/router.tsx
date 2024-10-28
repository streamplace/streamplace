import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { Provider } from "components";
import StreamList from "components/stream-list/stream-list";
import { registerRootComponent } from "expo";
import { Appearance, SafeAreaView } from "react-native";
import { useTheme } from "tamagui";

function HomeScreen() {
  return <StreamList></StreamList>;
}

const Stack = createNativeStackNavigator();

function App() {
  console.log(Appearance);
  // useEffect(() => {
  //   Appearance.setColorScheme("dark");
  // }, []);
  return (
    <Provider>
      <RenderArea />
    </Provider>
  );
}

const RenderArea = () => {
  const theme = useTheme();
  console.log(theme);
  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: theme.background.val }}>
      <Stack.Navigator>
        <Stack.Screen
          name="Home"
          component={HomeScreen}
          options={{ headerShown: true, headerTitle: "Aquareum" }}
        />
        {/* <Stack.Screen
          name="(tabs)"
          options={{
            title: "",
            headerShown: true,
            headerRight: () => {
              return (
                <Link href="/settings" asChild>
                  <Button icon={<Settings size="$2" />}></Button>
                </Link>
              );
            },
            headerLeft: () => (
              <Anchor href="https://explorer.livepeer.org/treasury/74518185892381909671177921640414850443801430499809418110611019961553289709442">
                <View bg="$accentColor" br="$5" padding="$2">
                  <H4 fontSize="$4">What's Aquareum?</H4>
                </View>
              </Anchor>
            ),
          }}
        /> */}
      </Stack.Navigator>
    </SafeAreaView>
  );
};

registerRootComponent(App);

export default App;
