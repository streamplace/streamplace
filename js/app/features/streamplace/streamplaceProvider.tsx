import { Text } from "@streamplace/components";
import Loading from "components/loading/loading";
import { createContext, useEffect } from "react";
import { View } from "react-native";
import { useStore } from "store";
import { useStreamplaceInitialized, useStreamplaceUrl } from "store/hooks";
import { DEFAULT_URL } from "store/slices/streamplaceSlice";

export const StreamplaceContext = createContext({
  url: DEFAULT_URL,
});

export default function StreamplaceProvider({
  children,
}: {
  children: React.ReactNode;
}): React.ReactElement {
  const url = useStreamplaceUrl();
  const initialized = useStreamplaceInitialized();
  const initialize = useStore((state) => state.initialize);

  useEffect(() => {
    if (!initialized) {
      initialize();
    }
  }, [initialized]);

  if (!initialized) {
    return (
      <View style={[{ flex: 1 }]}>
        <Text style={[{ color: "#fff" }]}>StreamplaceProvider loading...</Text>
        <Loading />
      </View>
    );
  }

  return (
    <StreamplaceContext.Provider value={{ url: url }}>
      {children}
    </StreamplaceContext.Provider>
  );
}
