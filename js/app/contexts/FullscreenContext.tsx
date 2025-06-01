import {
  setSidebarHidden,
  setSidebarUnhidden,
} from "features/base/sidebarSlice";
import {
  createContext,
  ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";
import { useDispatch } from "react-redux";

interface FullscreenContextValue {
  fullscreen: boolean;
  setFullscreen: (value: boolean) => void;
}

const FullscreenContext = createContext<FullscreenContextValue | undefined>(
  undefined,
);

export const FullscreenProvider = ({ children }: { children: ReactNode }) => {
  const [fullscreen, setFullscreen] = useState(false);

  // for hiding sidebar
  const dispatch = useDispatch();

  useEffect(() => {
    if (fullscreen) {
      dispatch(setSidebarHidden());
    } else {
      dispatch(setSidebarUnhidden());
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
