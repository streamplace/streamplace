import { responsiveValue } from "@streamplace/components/src/lib/utils";
import { useMemo } from "react";
import { useWindowDimensions } from "react-native";
import { SharedValue } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

export interface ResponsiveLayoutConfig {
  screenWidth: number;
  screenHeight: number;
  isLandscape: boolean;
  isMobile: boolean;
  isTablet: boolean;
  isDesktop: boolean;
  shouldShowFloatingMetrics: boolean;
  shouldShowBottomMetadata: boolean;
  shouldShowChatSidePanel: boolean;
  shouldShowChatOverlay: boolean;
  chatPanelWidth: number;
  safeAreaInsets: {
    top: number;
    bottom: number;
    left: number;
    right: number;
  };
}

export function useResponsiveLayout({
  sidebarWidth = 0,
  sidebarHidden = true,
  showChatSidePanelOnLandscape = true,
}: {
  sidebarWidth?: number | SharedValue<number>;
  sidebarHidden?: boolean;
  showChatSidePanelOnLandscape?: boolean;
} = {}): ResponsiveLayoutConfig & { contentWidth: number } {
  const { width: screenWidth, height: screenHeight } = useWindowDimensions();
  const safeAreaInsets = useSafeAreaInsets();

  const sidebarWidthValue = useMemo(() => {
    if (typeof sidebarWidth === "object" && "value" in sidebarWidth) {
      return sidebarWidth.value;
    }
    return sidebarWidth;
  }, [sidebarWidth]);

  const layout = useMemo(() => {
    const isLandscape = screenWidth > screenHeight;
    const isMobile = screenWidth < 768;
    const isTablet = screenWidth >= 768 && screenWidth < 980;
    const isDesktop = screenWidth >= 980;

    const shouldShowFloatingMetrics = isMobile;
    const shouldShowBottomMetadata = isDesktop;
    const shouldShowChatSidePanel =
      isLandscape && screenWidth >= 768 && showChatSidePanelOnLandscape;
    const shouldShowChatOverlay = !(isLandscape && screenWidth >= 768);

    const chatPanelWidth = responsiveValue(
      {
        md: 320,
        lg: 400,
        xl: 480,
        default: 300,
      },
      screenWidth,
    );

    const contentWidth =
      !sidebarHidden && sidebarWidthValue > 0
        ? screenWidth - sidebarWidthValue
        : screenWidth;

    return {
      screenWidth,
      screenHeight,
      isLandscape,
      isMobile,
      isTablet,
      isDesktop,
      shouldShowFloatingMetrics,
      shouldShowBottomMetadata,
      shouldShowChatSidePanel,
      shouldShowChatOverlay,
      chatPanelWidth,
      contentWidth,
    };
  }, [
    screenWidth,
    screenHeight,
    sidebarWidthValue,
    sidebarHidden,
    showChatSidePanelOnLandscape,
  ]);

  return {
    ...layout,
    safeAreaInsets,
  };
}
