import { responsiveValue } from "@streamplace/components/src/lib/utils";
import { useMemo } from "react";
import { useWindowDimensions } from "react-native";
import { SharedValue } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

export interface ResponsiveLayoutConfig {
  shouldShowChatSidePanel: boolean;
  shouldShowFloatingMetrics: boolean;
  chatPanelWidth: number;
  safeAreaInsets: {
    top: number;
    bottom: number;
    left: number;
    right: number;
  };
  screenWidth: number;
  availableHeight: number;
}

export function useResponsiveLayout({
  sidebarWidth = 0,
  sidebarHidden = true,
  showChatSidePanelOnLandscape = true,
}: {
  sidebarWidth?: number | SharedValue<number>;
  sidebarHidden?: boolean;
  showChatSidePanelOnLandscape?: boolean;
} = {}): ResponsiveLayoutConfig & {
  contentWidth: number;
} {
  const { width: screenWidth, height: screenHeight } = useWindowDimensions();
  const safeAreaInsets = useSafeAreaInsets();

  const sidebarWidthValue = useMemo(() => {
    if (typeof sidebarWidth === "object" && "value" in sidebarWidth) {
      return sidebarWidth.value;
    }
    return sidebarWidth;
  }, [sidebarWidth]);

  const isLandscape = screenWidth > screenHeight;
  const shouldShowChatSidePanel =
    isLandscape && screenWidth >= 768 && showChatSidePanelOnLandscape;

  const shouldShowFloatingMetrics = screenWidth < 768;
  const availableHeight =
    screenHeight - safeAreaInsets.top - safeAreaInsets.bottom;

  const chatPanelWidth = responsiveValue(
    {
      md: 320,
      lg: 400,
      xl: 480,
      default: 300,
    },
    screenWidth,
  );

  const availableWidth =
    screenWidth - safeAreaInsets.left - safeAreaInsets.right / 2;

  const contentWidth =
    !sidebarHidden && sidebarWidthValue > 0
      ? availableWidth - sidebarWidthValue
      : availableWidth;

  return {
    shouldShowChatSidePanel,
    shouldShowFloatingMetrics,
    chatPanelWidth,
    safeAreaInsets,
    contentWidth,
    screenWidth,
    availableHeight,
  };
}
