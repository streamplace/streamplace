import { useEffect } from "react";
import { Platform } from "react-native";
import { useSiteTitle } from "../streamplace-store";

/**
 * Hook to set the document title on web based on branding.
 * No-op on native platforms.
 */
export function useDocumentTitle() {
  const siteTitle = useSiteTitle();

  useEffect(() => {
    if (Platform.OS === "web" && typeof document !== "undefined") {
      document.title = siteTitle;
    }
  }, [siteTitle]);
}
