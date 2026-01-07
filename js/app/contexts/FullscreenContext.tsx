import {
  createContext,
  ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";
import { useStore } from "store";

interface FullscreenContextValue {
  fullscreen: boolean;
  setFullscreen: (value: boolean) => void;
}

const FullscreenContext = createContext<FullscreenContextValue | undefined>(
  undefined,
);

export const FullscreenProvider = ({ children }: { children: ReactNode }) => {
  const [fullscreen, setFullscreen] = useState(false);

  const setSidebarHidden = useStore((state) => state.setSidebarHidden);
  const setSidebarUnhidden = useStore((state) => state.setSidebarUnhidden);

  useEffect(() => {
    if (fullscreen) {
      setSidebarHidden();
    } else {
      setSidebarUnhidden();
    }
  }, [fullscreen]);

  return (
    <FullscreenContext.Provider value={{ fullscreen, setFullscreen }}>
      {children}
    </FullscreenContext.Provider>
  );
};

export function useFullscreen() {
  const ctx = useContext(FullscreenContext);
  if (!ctx) {
    throw new Error("useFullscreen must be used within a FullscreenProvider");
  }
  return ctx;
}
