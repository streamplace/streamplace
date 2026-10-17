// True when the viewport is wide enough for the desktop sidebar /
// multi-column layouts. The app's useIsLargeScreen (>= 980px) drives
// the native shell's tab bar visibility; on web it gates the sidebar
// overlay and tab-style layouts.
//
// Reanimated's useWindowDimensions isn't used here; we just listen
// to window resize.
import { useEffect, useState } from "react";

const LARGE_SCREEN_BREAKPOINT = 980;

export function useIsLargeScreen(): boolean {
  const [isLarge, setIsLarge] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return window.innerWidth >= LARGE_SCREEN_BREAKPOINT;
  });

  useEffect(() => {
    if (typeof window === "undefined") return;
    const onResize = () => {
      setIsLarge(window.innerWidth >= LARGE_SCREEN_BREAKPOINT);
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  return isLarge;
}
