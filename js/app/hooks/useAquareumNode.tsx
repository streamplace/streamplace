import { createContext, useContext, useState } from "react";
import { isWeb } from "tamagui";

let DEFAULT_URL = process.env.EXPO_PUBLIC_AQUAREUM_URL;
console.log({
  EXPO_PUBLIC_WEB_TRY_LOCAL: process.env.EXPO_PUBLIC_WEB_TRY_LOCAL,
});
if (isWeb && process.env.EXPO_PUBLIC_WEB_TRY_LOCAL === "true") {
  try {
    DEFAULT_URL = `${window.location.protocol}//${window.location.host}`;
  } catch (err) {
    // Oh well, fall back to hardcoded.
  }
}

export const AquareumContext = createContext({
  url: DEFAULT_URL,
  setUrl: (_: string) => {},
});

export function AquareumProvider({
  url: providedUrl,
  children,
}: {
  url?: string;
  children: React.ReactNode;
}) {
  const [url, setUrl] = useState(providedUrl || DEFAULT_URL);
  const val = { url, setUrl };
  return (
    <AquareumContext.Provider value={val}>{children}</AquareumContext.Provider>
  );
}

export default function useAquareumNode() {
  return useContext(AquareumContext);
}
