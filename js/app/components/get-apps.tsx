import { Image } from "expo-image";
import { Linking, TouchableOpacity, View } from "react-native";

const RATIO = 3.39741547176;
const WIDTH = 200;
const HEIGHT = 200 / RATIO;

export default function GetApps() {
  return (
    <View style={[{ justifyContent: "center" }, { flexDirection: "row" }]}>
      <TouchableOpacity
        onPress={() =>
          Linking.openURL(
            "https://apps.apple.com/us/app/streamplace/id6535653195",
          )
        }
      >
        <Image
          style={[{ width: WIDTH, height: HEIGHT, marginHorizontal: 8 }]}
          source={require("../assets/images/appstore.svg")}
        />
      </TouchableOpacity>
      <TouchableOpacity
        onPress={() =>
          Linking.openURL(
            "https://play.google.com/store/apps/details?id=tv.aquareum",
          )
        }
      >
        <Image
          style={[{ width: WIDTH, height: HEIGHT, marginHorizontal: 8 }]}
          source={require("../assets/images/playstore.svg")}
        />
      </TouchableOpacity>
    </View>
  );
}
