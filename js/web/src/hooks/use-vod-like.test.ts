import { describe, expect, it } from "vitest";
import { findViewerLike, type LikeRecordPage } from "./use-vod-like";

describe("findViewerLike", () => {
  it("follows pagination until it finds the viewer's like", async () => {
    const pages: Record<string, LikeRecordPage> = {
      first: {
        cursor: "second",
        records: [
          {
            uri: "at://did:plc:viewer/place.stream.like/one",
            value: { subject: "at://did:plc:author/place.stream.video/other" },
          },
        ],
      },
      second: {
        records: [
          {
            uri: "at://did:plc:viewer/place.stream.like/two",
            value: { subject: "at://did:plc:author/place.stream.video/wanted" },
          },
        ],
      },
    };
    const visited: Array<string | undefined> = [];

    const result = await findViewerLike(async (cursor) => {
      visited.push(cursor);
      return pages[cursor ?? "first"];
    }, "at://did:plc:author/place.stream.video/wanted");

    expect(result).toBe("at://did:plc:viewer/place.stream.like/two");
    expect(visited).toEqual([undefined, "second"]);
  });

  it("returns null after exhausting the viewer's likes", async () => {
    const result = await findViewerLike(async () => ({ records: [] }), "video");

    expect(result).toBeNull();
  });
});
