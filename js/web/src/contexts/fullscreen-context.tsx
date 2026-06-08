// Fullscreen state for the player. Port of
// js/app/contexts/FullscreenContext.tsx. The app's version hides the
// custom sidebar on fullscreen via a reanimated side effect; the
// web's shadcn sidebar manages its own state, so the context just
// exposes a boolean + setter. Future fullscreen UI can subscribe.
import {
  createContext,
  ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";

interface FullscreenContextValue {
  fullscreen: boolean;
  setFullscreen: (value: boolean) => void;
}

const FullscreenContext = createContext<FullscreenContextValue | undefined>(
  undefined,
);

export const FullscreenProvider = ({ children }: { children: ReactNode }) => {
  const [fullscreen, setFullscreen] = useState(false);

  useEffect(() => {
    // Browser-level fullscreen state is the source of truth on web;
    // sync our local state when the user hits Esc.
    if (typeof document === "undefined") return;
    const onChange = () => {
      setFullscreen(!!document.fullscreenElement);
    };
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

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
