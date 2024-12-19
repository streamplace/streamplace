import { useEffect, useState } from "react";
import { View, Text } from "tamagui";
import { generatePrivateKey } from "viem/accounts";

export default function StreamKeyScreen() {
  const [privateKey, setPrivateKey] = useState<`0x${string}` | null>(null);
  useEffect(() => {
    const privateKey = generatePrivateKey();
    setPrivateKey(privateKey);
  }, []);
  return (
    <View>
      <Text>{privateKey}</Text>
    </View>
  );
}
