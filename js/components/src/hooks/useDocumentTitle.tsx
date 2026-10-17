import { useEffect } from "react";
import { Platform } from "react-native";
import {
  useBrandingAsset,
  useFavicon,
  useSiteDescription,
  useSiteTitle,
} from "../streamplace-store";

/**
 * Hook to set the document title, description and browser favicons from branding.
 * No-op on native platforms. Favicon media queries follow the browser/OS scheme,
 * not the app's theme setting. Re-assert links after hydration because Expo can
 * drop the static links.
 */
export function useDocumentTitle() {
  const siteTitle = useSiteTitle();
  const siteDescription = useSiteDescription();
  const favicon = useFavicon();
  const faviconLight = useBrandingAsset("faviconLight")?.data;
  const faviconDark = useBrandingAsset("faviconDark")?.data;

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
    }
  }, [siteTitle, siteDescription]);

  useEffect(() => {
    if (Platform.OS !== "web" || typeof document === "undefined") return;

    // A branding change can clear the last upload. Use fresh fallback URLs so
    // the browser cannot resurrect an image cached from the initial HTML.
    const fallback = `/favicon.png?v=${Date.now()}`;

    // Generic first: browsers prefer the last icon whose media query matches.
    const icons = [
      { href: favicon || fallback, media: "" },
      {
        href: faviconLight || favicon || `${fallback}&scheme=light`,
        media: "(prefers-color-scheme: light)",
      },
      {
        href: faviconDark || favicon || `${fallback}&scheme=dark`,
        media: "(prefers-color-scheme: dark)",
      },
    ];
    const links = Array.from(
      document.head.querySelectorAll<HTMLLinkElement>('link[rel="icon"]'),
    );
    icons.forEach(({ href, media }, index) => {
      const link = links[index] || document.createElement("link");
      link.rel = "icon";
      // The response/data URL declares the uploaded format (PNG, SVG or ICO).
      link.removeAttribute("type");
      link.href = href;
      link.media = media;
      if (!link.parentNode) document.head.appendChild(link);
    });
    links.slice(icons.length).forEach((link) => link.remove());
  }, [favicon, faviconLight, faviconDark]);
}
