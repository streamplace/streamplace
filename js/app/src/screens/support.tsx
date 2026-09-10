import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import { View } from "react-native";

export default function SupportScreen() {
  const { isWeb } = usePlatform();
  useEffect(() => {
    if (!isWeb) return;
    document.location.href =
      "https://docs.google.com/forms/d/14ATDKwOkSN1SDxb_anMT1iafs3JtyXSoubSBEoJuA5g/edit";
  }, [isWeb]);
  return <View />;
}
