import type { Orientation } from "expo-screen-orientation";
import React, {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  isRotationAvailable,
  ScreenOrientation,
} from "./rotation-async.native";

export interface RotationContextValue {
  currentOrientation: Orientation;
  isLocked: boolean;
  isActive: boolean;
  rotateToLandscape: () => Promise<void>;
  rotateToPortrait: () => Promise<void>;
  toggleRotation: () => Promise<void>;
  canRotate: boolean;
}

export interface RotationProviderProps {
  children: React.ReactNode;
  enabled?: boolean;
}

const RotationContext = createContext<RotationContextValue | null>(null);

export const RotationProvider: React.FC<RotationProviderProps> = ({
  children,
  enabled = true,
}) => {
  const [isLocked, setIsLocked] = useState(false);
  const [canRotate, setCanRotate] = useState(isRotationAvailable);
  const [currentOrientation, setCurrentOrientation] = useState<Orientation>(
    ScreenOrientation?.Orientation.PORTRAIT_UP ?? 1,
  );

  // If module not available, provide disabled context
  if (!isRotationAvailable || !ScreenOrientation) {
    const disabledContextValue: RotationContextValue = {
      currentOrientation: 1, // Orientation.PORTRAIT_UP
      isLocked: false,
      isActive: false,
      rotateToLandscape: async () => {},
      rotateToPortrait: async () => {},
      toggleRotation: async () => {},
      canRotate: false,
    };

    return (
      <RotationContext.Provider value={disabledContextValue}>
        {children}
      </RotationContext.Provider>
    );
  }

  // Manual rotation functions
  const rotateToLandscape = async () => {
    if (!enabled || !canRotate || !ScreenOrientation) return;

    try {
      await ScreenOrientation.unlockAsync();
      await ScreenOrientation.lockAsync(
        ScreenOrientation.OrientationLock.LANDSCAPE_RIGHT,
      );
      setIsLocked(true);

      // set current orientation to landscape right
      setCurrentOrientation(ScreenOrientation.Orientation.LANDSCAPE_RIGHT);

      if (__DEV__) {
        console.log("📲 Manual landscape");
      }
    } catch (error) {
      console.warn("Failed to rotate to landscape:", error);
    }
  };

  const rotateToPortrait = async () => {
    if (!enabled || !canRotate || !ScreenOrientation) return;

    try {
      await ScreenOrientation.unlockAsync();
      await ScreenOrientation.lockAsync(
        ScreenOrientation.OrientationLock.PORTRAIT_UP,
      );
      setIsLocked(true);

      setCurrentOrientation(ScreenOrientation.Orientation.PORTRAIT_UP);

      if (__DEV__) {
        console.log("📲 Manual portrait");
      }
    } catch (error) {
      console.warn("Failed to rotate to portrait:", error);
    }
  };

  const toggleRotation = async () => {
    if (!ScreenOrientation) return;

    const isLandscape =
      currentOrientation === ScreenOrientation.Orientation.LANDSCAPE_LEFT ||
      currentOrientation === ScreenOrientation.Orientation.LANDSCAPE_RIGHT;

    if (__DEV__) {
      console.log(
        `🔄 Toggle: current=${currentOrientation}, isLandscape=${isLandscape}`,
      );
    }

    if (isLandscape) {
      await rotateToPortrait();
    } else {
      await rotateToLandscape();
    }
  };

  // Track orientation changes
  useEffect(() => {
    if (!enabled) return;

    const getCurrentOrientation = async () => {
      if (!ScreenOrientation) return;
      try {
        const orient = await ScreenOrientation.getOrientationAsync();
        setCurrentOrientation(orient);

        if (__DEV__) {
          console.log(`📲 Orientation on load: ${orient}`);
        }
      } catch (error) {
        console.warn("Failed to get orientation:", error);
      }
    };

    getCurrentOrientation();

    if (!ScreenOrientation) return;

    const subscription = ScreenOrientation.addOrientationChangeListener(
      (event) => {
        const newOrientation = event.orientationInfo.orientation;
        setCurrentOrientation(newOrientation);

        if (__DEV__) {
          console.log(
            `🔄 Orientation: ${newOrientation} (locked: ${isLocked})`,
          );
        }
      },
    );

    return () => {
      subscription?.remove();
      if (ScreenOrientation) {
        ScreenOrientation.removeOrientationChangeListeners();
        ScreenOrientation.unlockAsync().catch(() => {});
      }
    };
  }, [enabled]);

  const contextValue = useMemo(
    () => ({
      currentOrientation,
      isLocked,
      isActive: enabled,
      rotateToLandscape,
      rotateToPortrait,
      toggleRotation,
      canRotate,
    }),
    [
      currentOrientation,
      isLocked,
      enabled,
      canRotate,
      rotateToLandscape,
      rotateToPortrait,
      toggleRotation,
    ],
  );

  return (
    <RotationContext.Provider value={contextValue}>
      {children}
    </RotationContext.Provider>
  );
};

export const useRotation = (): RotationContextValue => {
  const context = useContext(RotationContext);
  if (!context) {
    throw new Error("useRotation must be used within a RotationProvider");
  }
  return context;
};
