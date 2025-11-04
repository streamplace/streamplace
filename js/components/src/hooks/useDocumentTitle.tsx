import { useEffect } from "react";
import { Platform } from "react-native";
import {
  useFavicon,
  useSiteDescription,
  useSiteTitle,
} from "../streamplace-store";

/**
 * Hook to set the document title, description, and favicon on web based on branding.
 * No-op on native platforms.
 */
export function useDocumentTitle() {
  const siteTitle = useSiteTitle();
  const siteDescription = useSiteDescription();
  const favicon = useFavicon();

  useEffect(() => {
    if (Platform.OS === "web" && typeof document !== "undefined") {
      // set title
      document.title = siteTitle;

      // set or update meta description
      let metaDescription = document.querySelector('meta[name="description"]');
      if (!metaDescription) {
        metaDescription = document.createElement("meta");
        metaDescription.setAttribute("name", "description");
        document.head.appendChild(metaDescription);
      }
      metaDescription.setAttribute("content", siteDescription);

      // set or update favicon
      if (favicon) {
        let link: HTMLLinkElement | null =
          document.querySelector('link[rel="icon"]');
        if (!link) {
          link = document.createElement("link");
          link.rel = "icon";
          document.head.appendChild(link);
        }
        link.href = favicon;
      }
    }
  }, [siteTitle, siteDescription, favicon]);
}
