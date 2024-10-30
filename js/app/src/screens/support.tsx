import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import { View } from "react-native";

export default function SupportScreen() {
  const { isWeb } = usePlatform();
  if (isWeb) {
    useEffect(() => {
      document.location.href =
        "https://docs.google.com/forms/d/14ATDKwOkSN1SDxb_anMT1iafs3JtyXSoubSBEoJuA5g/edit";
    }, []);
  }
  return <View />;
}
