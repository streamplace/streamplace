import { useEffect } from "react";
import { Platform } from "react-native";
import { useSiteDescription, useSiteTitle } from "../streamplace-store";

/**
 * Hook to set the document title and description on web based on branding.
 * No-op on native platforms.
 *
 * The tab favicon is intentionally owned by the app's bundled icon
 * (public/index.html + app.config `favicon`) and is re-asserted here rather
 * than driven by branding: letting a node's stored branding favicon override
 * it replaced the current Streamplace mark with whatever (often stale) icon
 * the node had on file. Keeping the bundled mark authoritative guarantees the
 * brand stays consistent; white-label deployments set their favicon at build
 * time. We re-assert it because Expo/React can drop the static <link> during
 * hydration, which would otherwise leave the tab with no icon.
 */
export function useDocumentTitle() {
  const siteTitle = useSiteTitle();
  const siteDescription = useSiteDescription();

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

      // keep the bundled Streamplace mark as the tab favicon
      let link: HTMLLinkElement | null =
        document.querySelector('link[rel="icon"]');
      if (!link) {
        link = document.createElement("link");
        link.rel = "icon";
        link.type = "image/png";
        document.head.appendChild(link);
      }
      if (link.getAttribute("href") !== "/favicon.png") {
        link.setAttribute("href", "/favicon.png");
      }
    }
  }, [siteTitle, siteDescription]);
}
