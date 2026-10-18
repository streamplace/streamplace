import { place } from "streamplace";
import { describe, expect, it } from "vitest";
import { detectChatFacets } from "./facet";

const MENTION_DID = "did:plc:svzjldcfdhtytju6wa2vjrth";

// Only mention resolution reaches the agent.
const agent = {
  com: {
    atproto: {
      identity: {
        resolveHandle: async ({ handle }: { handle: string }) => ({
          data: {
            did: handle === "sidechannel.cheeky.town" ? MENTION_DID : "",
          },
        }),
      },
    },
  },
} as unknown as Parameters<typeof detectChatFacets>[0];

describe("detectChatFacets", () => {
  it("types link facets, which detectFacets leaves untyped", async () => {
    const facets = await detectChatFacets(
      agent,
      "read through the dist.town/release.software docs?",
    );

    expect(facets).toHaveLength(1);
    expect(facets?.[0]).toMatchObject({
      $type: "place.stream.richtext.facet",
      index: { byteStart: 17, byteEnd: 43 },
      features: [
        {
          $type: "app.bsky.richtext.facet#link",
          uri: "https://dist.town/release.software",
        },
      ],
    });
    // Any other facet $type is rejected, and the record with it discarded.
    expect(
      place.stream.richtext.facet.main.$safeValidate(facets?.[0]).success,
    ).toBe(true);
  });

  it("restamps the app.bsky facet type detectFacets writes on mentions", async () => {
    const facets = await detectChatFacets(
      agent,
      "@sidechannel.cheeky.town hello!",
    );

    expect(facets?.[0]).toMatchObject({
      $type: "place.stream.richtext.facet",
      features: [
        { $type: "app.bsky.richtext.facet#mention", did: MENTION_DID },
      ],
    });
    expect(
      place.stream.richtext.facet.main.$safeValidate(facets?.[0]).success,
    ).toBe(true);
  });

  it("drops facets whose features the chat lexicons do not allow", async () => {
    expect(await detectChatFacets(agent, "see you at #lxiv")).toBeUndefined();
    expect(await detectChatFacets(agent, "no facets here")).toBeUndefined();
  });
});
