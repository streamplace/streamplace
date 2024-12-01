import { createContext, useEffect } from "react";
import { DEFAULT_URL, initialize, selectAquareum } from "./aquareumSlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import Loading from "components/loading/loading";
import { View, Text } from "tamagui";

export const AquareumContext = createContext({
  url: DEFAULT_URL,
});

export default function AquareumProvider({
  children,
}: {
  children: React.ReactNode;
}): React.ReactElement {
  const aquareum = useAppSelector(selectAquareum);
  const dispatch = useAppDispatch();
  useEffect(() => {
    if (!aquareum.initialized) {
      dispatch(initialize());
    }
  }, [aquareum.initialized]);
  if (!aquareum.initialized) {
    return (
      <View f={1}>
        <Text>AquareumProvider loading...</Text>
        <Loading />
      </View>
    );
  }
  return (
    <AquareumContext.Provider value={{ url: aquareum.url }}>
      {children}
    </AquareumContext.Provider>
  );
}
