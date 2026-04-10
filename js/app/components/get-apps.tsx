import { STORE_URLS } from "constants/store-urls";
import { Image } from "expo-image";
import { Linking, TouchableOpacity, View } from "react-native";

const RATIO = 3.39741547176;
const WIDTH = 200;
const HEIGHT = 200 / RATIO;

export default function GetApps() {
  return (
    <View style={[{ justifyContent: "center" }, { flexDirection: "row" }]}>
      <TouchableOpacity onPress={() => Linking.openURL(STORE_URLS.ios)}>
        <Image
          style={[{ width: WIDTH, height: HEIGHT, marginHorizontal: 8 }]}
          source={require("../assets/images/appstore.svg")}
        />
      </TouchableOpacity>
      <TouchableOpacity onPress={() => Linking.openURL(STORE_URLS.android)}>
        <Image
          style={[{ width: WIDTH, height: HEIGHT, marginHorizontal: 8 }]}
          source={require("../assets/images/playstore.svg")}
        />
      </TouchableOpacity>
    </View>
  );
}
