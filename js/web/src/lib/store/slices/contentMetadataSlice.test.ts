import { describe, expect, it, vi } from "vitest";
import type { AppStore } from "../index";
import {
  createContentMetadataSlice,
  type ContentMetadataSlice,
} from "./contentMetadataSlice";

type RecordData = {
  captionPolicy?: { canonical: string; allowNodeCaptions?: boolean };
  contentRights?: { creator?: string; copyrightNotice?: string };
  distributionPolicy?: { allowGenAiTraining?: boolean; deleteAfter?: number };
  contentWarnings?: { warnings: string[] };
};

describe("metadata caption policy edits", () => {
  it.each(["createContentMetadata", "updateContentMetadata"] as const)(
    "%s preserves unedited rights and distribution settings",
    async (action) => {
      let written: RecordData = {};
      const state: Partial<AppStore> = {};
      const slice = createContentMetadataSlice(
        (next) =>
          Object.assign(
            state,
            typeof next === "function" ? next(state as AppStore) : next,
          ),
        () => state as AppStore,
        {} as Parameters<typeof createContentMetadataSlice>[2],
      );
      Object.assign(state, slice, {
        oauthSession: { did: "did:plc:owner" },
        pdsAgent: {
          com: {
            atproto: {
              repo: {
                putRecord: vi.fn(async ({ record }: { record: RecordData }) => {
                  written = record;
                  return {
                    data: {
                      uri: "at://did:plc:owner/place.stream.metadata.configuration/self",
                      cid: "cid",
                    },
                  };
                }),
              },
            },
          },
        },
        lastCreatedRecord: {
          record: {
            $type: "place.stream.metadata.configuration",
            contentRights: {
              creator: "Original Creator",
              copyrightNotice: "Original notice",
            },
            distributionPolicy: { allowGenAiTraining: false, deleteAfter: 90 },
            contentWarnings: { warnings: ["violence"] },
          },
        },
      });
      await (state as ContentMetadataSlice)[action]({
        captionPolicy: { canonical: "off", allowNodeCaptions: false },
        contentRights: { copyrightNotice: "Edited notice" },
        distributionPolicy: { deleteAfter: 120 },
      });
      expect(written.captionPolicy?.canonical).toBe("off");
      expect(written.contentRights).toEqual({
        creator: "Original Creator",
        copyrightNotice: "Edited notice",
      });
      expect(written.distributionPolicy).toEqual({
        allowGenAiTraining: false,
        deleteAfter: 120,
      });
      expect(written.contentWarnings).toEqual({ warnings: ["violence"] });
    },
  );
});
