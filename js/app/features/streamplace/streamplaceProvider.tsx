import { createContext, useEffect } from "react";
import { DEFAULT_URL, initialize, selectStreamplace } from "./streamplaceSlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import Loading from "components/loading/loading";
import { View, Text } from "tamagui";

export const StreamplaceContext = createContext({
  url: DEFAULT_URL,
});

export default function StreamplaceProvider({
  children,
}: {
  children: React.ReactNode;
}): React.ReactElement {
  const streamplace = useAppSelector(selectStreamplace);
  const dispatch = useAppDispatch();
  useEffect(() => {
    if (!streamplace.initialized) {
      dispatch(initialize());
    }
  }, [streamplace.initialized]);
  if (!streamplace.initialized) {
    return (
      <View f={1}>
        <Text>StreamplaceProvider loading...</Text>
        <Loading />
      </View>
    );
  }
  return (
    <StreamplaceContext.Provider value={{ url: streamplace.url }}>
      {children}
    </StreamplaceContext.Provider>
  );
}
