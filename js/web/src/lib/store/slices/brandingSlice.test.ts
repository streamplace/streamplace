import { afterEach, describe, expect, it, vi } from "vitest";
import { storage } from "../../storage";
import { createBrandingSlice } from "./brandingSlice";

function makeBrandingState(agent: unknown) {
  let state: Record<string, any> = {};
  const set = (next: any) => {
    Object.assign(state, typeof next === "function" ? next(state) : next);
  };
  const slice = createBrandingSlice(set as any, () => state as any, {} as any);
  Object.assign(state, slice, {
    broadcasterDID: "did:plc:broadcaster",
    url: "https://stream.place",
    pdsAgent: agent,
    anonPDSAgent: null,
  });
  return state as typeof slice;
}

describe("fetchBranding", () => {
  afterEach(() => vi.restoreAllMocks());

  it("reports a failed request after storing the error", async () => {
    const getBranding = vi.fn().mockRejectedValue(new Error("offline"));
    const state = makeBrandingState({ client: { call: getBranding } });
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    vi.spyOn(storage, "getItem").mockResolvedValue(null);

    const succeeded = await state.fetchBranding({ force: true });

    expect(succeeded).toBe(false);
    expect(state.brandingLoading).toBe(false);
    expect(state.brandingError).toBe("offline");
    expect(consoleError).toHaveBeenCalledWith(
      "Failed to fetch branding:",
      expect.any(Error),
    );
  });
});
