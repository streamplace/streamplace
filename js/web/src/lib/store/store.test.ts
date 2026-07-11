// Catch duplicate action keys across slices. When slices are spread
// into a flat store, a later slice silently overwrites an earlier
// slice's action with the same name. This test introspects the
// creator functions to detect collisions before they ship.
import { describe, expect, it } from "vitest";
import { createBaseSlice } from "./slices/baseSlice";
import { createBlueskySlice } from "./slices/blueskySlice";
import { createContentMetadataSlice } from "./slices/contentMetadataSlice";
import { createSidebarSlice } from "./slices/sidebarSlice";
import { createStreamplaceSlice } from "./slices/streamplaceSlice";

// Extract the top-level keys a slice creator would set on the store.
// We call each creator with stub set/get and collect the keys.
function getSliceKeys(
  creator: (set: any, get: any, api: any) => Record<string, unknown>,
): string[] {
  const obj = creator(
    () => {},
    () => ({}),
    {} as any,
  );
  return Object.keys(obj);
}

describe("store slice composition", () => {
  it("no two slices define the same key", () => {
    const slices = [
      { name: "base", keys: getSliceKeys(createBaseSlice as any) },
      { name: "sidebar", keys: getSliceKeys(createSidebarSlice as any) },
      {
        name: "streamplace",
        keys: getSliceKeys(createStreamplaceSlice as any),
      },
      { name: "bluesky", keys: getSliceKeys(createBlueskySlice as any) },
      {
        name: "contentMetadata",
        keys: getSliceKeys(createContentMetadataSlice as any),
      },
    ];

    const seen = new Map<string, string>(); // key -> slice name
    const duplicates: string[] = [];

    for (const slice of slices) {
      for (const key of slice.keys) {
        if (seen.has(key)) {
          duplicates.push(
            `"${key}" defined in both "${seen.get(key)}" and "${slice.name}"`,
          );
        } else {
          seen.set(key, slice.name);
        }
      }
    }

    expect(duplicates).toEqual([]);
  });
});
