import { useCallback, useEffect } from "react";
import { place } from "streamplace";
import storage from "../storage";
import {
  getStreamplaceStoreFromContext,
  useStreamplaceStore,
} from "./streamplace-store";
import { usePossiblyUnauthedPDSAgent } from "./xrpc";

export interface BrandingAsset {
  key: string;
  mimeType: string;
  url?: string; // URL for images
  data?: string; // inline data for text, or base64 for images
  width?: number; // image width in pixels
  height?: number; // image height in pixels
}

// helper to convert blob to base64
const blobToBase64 = (blob: Blob): Promise<string> => {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => resolve(reader.result as string);
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
};

const PropsInHeader = [
  "siteTitle",
  "siteDescription",
  "primaryColor",
  "accentColor",
  "defaultStreamer",
  "mainLogo",
  "favicon",
  "sidebarBg",
  "legalLinks",
];

function getMetaContent(key: string): BrandingAsset | null {
  if (typeof window === "undefined" || !window.document) return null;
  const meta = document.querySelector(`meta[name="internal-brand:${key}`);
  if (meta && meta.getAttribute("content")) {
    let content = meta.getAttribute("content");
    if (content) return JSON.parse(content) as BrandingAsset;
  }

  return null;
}

// hook to fetch broadcaster DID (unauthenticated)
export function useFetchBroadcasterDID() {
  const streamplaceAgent = usePossiblyUnauthedPDSAgent();
  const store = getStreamplaceStoreFromContext();

  // prefetch from meta records, if on web
  useEffect(() => {
    if (typeof window !== "undefined" && window.document) {
      try {
        const metaRecords = PropsInHeader.reduce(
          (acc, key) => {
            const meta = document.querySelector(
              `meta[name="internal-brand:${key}`,
            );
            // hrmmmmmmmmmmmm
            if (meta && meta.getAttribute("content")) {
              let content = meta.getAttribute("content");
              if (content) acc[key] = JSON.parse(content) as BrandingAsset;
            }
            return acc;
          },
          {} as Record<string, BrandingAsset>,
        );

        console.log("Found meta records for broadcaster DID:", metaRecords);
        // filter out all non-text values, can get on second fetch?
        for (const key of Object.keys(metaRecords)) {
          if (metaRecords[key].mimeType != "text/plain") {
            delete metaRecords[key];
          }
        }
      } catch (e) {
        console.warn("Failed to parse broadcaster DID from meta tags", e);
      }
    }
  }, []);

  return useCallback(async () => {
    try {
      if (!streamplaceAgent) {
        throw new Error("Streamplace agent not available");
      }
      const result = await streamplaceAgent.client.call(
        place.stream.broadcast.getBroadcaster,
      );
      store.setState({ broadcasterDID: result.broadcaster });
      if (result.server) {
        store.setState({ serverDID: result.server });
      }
      if (result.admins) {
        store.setState({ adminDIDs: result.admins });
      }
    } catch (err) {
      console.error("Failed to fetch broadcaster DID:", err);
    }
  }, [streamplaceAgent, store]);
}

// hook to fetch client-facing env config from the server
export function useFetchEnvConfig() {
  const streamplaceAgent = usePossiblyUnauthedPDSAgent();
  const store = getStreamplaceStoreFromContext();

  return useCallback(async () => {
    try {
      if (!streamplaceAgent) {
        throw new Error("Streamplace agent not available");
      }
      const result = await streamplaceAgent.client.call(
        place.stream.config.getEnv,
      );
      if (result.playbackWorkerUrl) {
        store.setState({ playbackWorkerUrl: result.playbackWorkerUrl });
      }
      store.setState({ gamesEnabled: result.gamesEnabled ?? false });
    } catch (err) {
      console.error("Failed to fetch env config:", err);
    }
  }, [streamplaceAgent, store]);
}

// hook to fetch branding data from the server
export function useFetchBranding() {
  const streamplaceAgent = usePossiblyUnauthedPDSAgent();
  const broadcasterDID = useStreamplaceStore((state) => state.broadcasterDID);
  const url = useStreamplaceStore((state) => state.url);
  const store = getStreamplaceStoreFromContext();

  return useCallback(
    async ({ force = true } = {}) => {
      if (!broadcasterDID) return;

      try {
        store.setState({ brandingLoading: true });

        // check localStorage first
        const cacheKey = `branding:${broadcasterDID}`;
        const cached = await storage.getItem(cacheKey);
        if (!force && cached) {
          try {
            const parsed = JSON.parse(cached);
            // check if cache is less than 1 hour old
            if (Date.now() - parsed.timestamp < 60 * 60 * 1000) {
              store.setState({
                branding: parsed.data,
                brandingLoading: false,
                brandingError: null,
              });
              return;
            }
          } catch (e) {
            // invalid cache, continue to fetch
            console.warn("Invalid branding cache, refetching", e);
          }
        }

        // fetch branding metadata from server
        if (!streamplaceAgent) {
          throw new Error("Streamplace agent not available");
        }
        const res = await streamplaceAgent.client.call(
          place.stream.branding.getBranding,
          {
            broadcaster: broadcasterDID as any,
          },
        );
        const assets = res.assets;

        // convert assets array to keyed object and fetch blob data
        const brandingMap: Record<string, BrandingAsset> = {};

        for (const asset of assets) {
          brandingMap[asset.key] = { ...asset };

          // if data is already inline (text assets), use it directly
          if (asset.data) {
            brandingMap[asset.key].data = asset.data;
          } else if (asset.url) {
            // for images, construct full URL and fetch blob
            const fullUrl = `${url}${asset.url}`;
            const blobRes = await fetch(fullUrl);
            const blob = await blobRes.blob();
            brandingMap[asset.key].data = await blobToBase64(blob);
          }
        }

        // cache in localStorage
        storage.setItem(
          cacheKey,
          JSON.stringify({
            timestamp: Date.now(),
            data: brandingMap,
          }),
        );

        store.setState({
          branding: brandingMap,
          brandingLoading: false,
          brandingError: null,
        });
      } catch (err: any) {
        console.error("Failed to fetch branding:", err);
        store.setState({
          brandingLoading: false,
          brandingError: err.message || "Failed to fetch branding",
        });
      }
    },
    [broadcasterDID, streamplaceAgent, url, store],
  );
}

// hook to get a specific branding asset by key
export function useBrandingAsset(key: string): BrandingAsset | undefined {
  return (
    useStreamplaceStore((state) => state.branding?.[key]) ||
    getMetaContent(key) ||
    undefined
  );
}

// convenience hook for main logo
export function useMainLogo(): string | undefined {
  const asset = useBrandingAsset("mainLogo");
  return asset?.data;
}

// convenience hook for favicon
export function useFavicon(): string | undefined {
  const asset = useBrandingAsset("favicon");
  return asset?.data;
}

// convenience hook for site title
export function useSiteTitle(): string {
  const asset = useBrandingAsset("siteTitle");
  return asset?.data || "My Streamplace Station";
}

// convenience hook for site description
export function useSiteDescription(): string {
  const asset = useBrandingAsset("siteDescription");
  return asset?.data || "Live streaming platform";
}

// convenience hook for primary color
export function usePrimaryColor(): string {
  const asset = useBrandingAsset("primaryColor");
  return asset?.data || "#6366f1";
}

// convenience hook for accent color
export function useAccentColor(): string {
  const asset = useBrandingAsset("accentColor");
  return asset?.data || "#8b5cf6";
}

// convenience hook for default streamer
export function useDefaultStreamer(): string | undefined {
  const asset = useBrandingAsset("defaultStreamer");
  return asset?.data || undefined;
}

// convenience hook for sidebar background image
export function useSidebarBackgroundImage(): BrandingAsset | undefined {
  return useBrandingAsset("sidebarBackgroundImage");
}

// convenience hook for legal links
export function useLegalLinks(): { text: string; url: string }[] {
  const asset = useBrandingAsset("legalLinks");
  if (!asset?.data) {
    return [];
  }
  try {
    return JSON.parse(asset.data);
  } catch {
    return [];
  }
}

// hook to auto-fetch branding when broadcaster changes
export function useBrandingAutoFetch() {
  const fetchBranding = useFetchBranding();
  const broadcasterDID = useStreamplaceStore((state) => state.broadcasterDID);

  useEffect(() => {
    if (broadcasterDID) {
      fetchBranding();
    }
  }, [broadcasterDID, fetchBranding]);
}
