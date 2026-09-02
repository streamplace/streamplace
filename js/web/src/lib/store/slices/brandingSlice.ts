// Branding state + fetch. Ported from
// js/components/src/streamplace-store/branding.tsx (useFetchBroadcasterDID,
// useFetchBranding) so the advanced-settings "refresh branding" button and
// header site title work on web. Branding is per-broadcaster; the server
// injects the broadcaster DID via place.stream.broadcast.getBroadcaster and
// the assets come from place.stream.branding.getBranding.
import { place } from "streamplace";
import { StateCreator } from "zustand";
import { storage } from "../../storage";
import { AppStore } from "../index";

export interface BrandingAsset {
  key: string;
  mimeType: string;
  url?: string; // URL for images
  data?: string; // inline data for text, or base64 for images
  width?: number; // image width in pixels
  height?: number; // image height in pixels
}

const BRANDING_CACHE_TTL_MS = 60 * 60 * 1000; // 1 hour

// helper to convert blob to base64
function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => resolve(reader.result as string);
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
}

export interface BrandingSlice {
  broadcasterDID: string | null;
  serverDID: string | null;
  branding: Record<string, BrandingAsset> | null;
  brandingLoading: boolean;
  brandingError: string | null;
  fetchBroadcasterDID: () => Promise<void>;
  fetchBranding: (opts?: { force?: boolean }) => Promise<void>;
}

export const createBrandingSlice: StateCreator<
  AppStore,
  [],
  [],
  BrandingSlice
> = (set, get) => ({
  broadcasterDID: null,
  serverDID: null,
  branding: null,
  brandingLoading: false,
  brandingError: null,

  fetchBroadcasterDID: async () => {
    try {
      const agent = get().pdsAgent ?? get().anonPDSAgent;
      if (!agent) {
        throw new Error("Streamplace agent not available");
      }
      const result = await agent.client.call(
        place.stream.broadcast.getBroadcaster,
      );
      set({ broadcasterDID: result.broadcaster });
      if (result.server) {
        set({ serverDID: result.server });
      }
    } catch (err) {
      console.error("Failed to fetch broadcaster DID:", err);
    }
  },

  fetchBranding: async ({ force = true } = {}) => {
    const { broadcasterDID, url } = get();
    if (!broadcasterDID) {
      // If we don't know the broadcaster yet, resolve it first.
      await get().fetchBroadcasterDID();
      if (!get().broadcasterDID) return;
    }
    const did = get().broadcasterDID as string;

    try {
      set({ brandingLoading: true });

      // check localStorage first
      const cacheKey = `branding:${did}`;
      const cached = await storage.getItem(cacheKey);
      if (!force && cached) {
        try {
          const parsed = JSON.parse(cached);
          // check if cache is less than 1 hour old
          if (Date.now() - parsed.timestamp < BRANDING_CACHE_TTL_MS) {
            set({
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
      const agent = get().pdsAgent ?? get().anonPDSAgent;
      if (!agent) {
        throw new Error("Streamplace agent not available");
      }
      const res = await agent.client.call(place.stream.branding.getBranding, {
        broadcaster: did as any,
      });
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
      storage
        .setItem(
          cacheKey,
          JSON.stringify({
            timestamp: Date.now(),
            data: brandingMap,
          }),
        )
        .catch(console.error);

      set({
        branding: brandingMap,
        brandingLoading: false,
        brandingError: null,
      });
    } catch (err: any) {
      console.error("Failed to fetch branding:", err);
      set({
        brandingLoading: false,
        brandingError: err.message || "Failed to fetch branding",
      });
    }
  },
});
