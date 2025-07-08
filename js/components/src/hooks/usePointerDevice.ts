import { useEffect, useState } from "react";
import { Platform } from "react-native";

export interface PointerDevice {
  hasHover: boolean;
  hasFinePointer: boolean;
  isMouseDriven: boolean;
  isTouchDriven: boolean;
}

/**
 * Hook to detect if the device is primarily mouse-driven vs touch-driven
 * Uses CSS media queries to detect hover and pointer capabilities
 */
export function usePointerDevice(): PointerDevice {
  const [pointerDevice, setPointerDevice] = useState<PointerDevice>(() => {
    // Default values for non-web platforms
    if (Platform.OS !== "web") {
      return {
        hasHover: false,
        hasFinePointer: false,
        isMouseDriven: false,
        isTouchDriven: true,
      };
    }

    // Initial web detection
    if (typeof window !== "undefined" && window.matchMedia) {
      const hasHover = window.matchMedia("(hover: hover)").matches;
      const hasFinePointer = window.matchMedia("(pointer: fine)").matches;

      return {
        hasHover,
        hasFinePointer,
        isMouseDriven: hasHover && hasFinePointer,
        isTouchDriven: !hasHover || !hasFinePointer,
      };
    }

    // Fallback for SSR or environments without matchMedia
    return {
      hasHover: false,
      hasFinePointer: false,
      isMouseDriven: false,
      isTouchDriven: true,
    };
  });

  useEffect(() => {
    // Only run on web platforms
    if (
      Platform.OS !== "web" ||
      typeof window === "undefined" ||
      !window.matchMedia
    ) {
      return;
    }

    const hoverQuery = window.matchMedia("(hover: hover)");
    const pointerQuery = window.matchMedia("(pointer: fine)");

    const updatePointerDevice = () => {
      const hasHover = hoverQuery.matches;
      const hasFinePointer = pointerQuery.matches;

      setPointerDevice({
        hasHover,
        hasFinePointer,
        isMouseDriven: hasHover && hasFinePointer,
        isTouchDriven: !hasHover || !hasFinePointer,
      });
    };

    // Set up listeners for media query changes
    hoverQuery.addEventListener("change", updatePointerDevice);
    pointerQuery.addEventListener("change", updatePointerDevice);

    // Initial update
    updatePointerDevice();

    // Cleanup
    return () => {
      hoverQuery.removeEventListener("change", updatePointerDevice);
      pointerQuery.removeEventListener("change", updatePointerDevice);
    };
  }, []);

  return pointerDevice;
}
