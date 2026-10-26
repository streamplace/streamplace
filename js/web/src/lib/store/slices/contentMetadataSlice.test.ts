import { Agent } from "@atproto/api";
import type { StreamplaceAgent } from "streamplace";
import { describe, expect, it, vi } from "vitest";
import { createContentMetadataSlice } from "./contentMetadataSlice";

vi.mock("../../did", () => ({
  getPDSServiceEndpoint: vi.fn(),
  resolveDIDDocument: vi.fn(),
}));

type PutRecordInput = NonNullable<
  Parameters<StreamplaceAgent["com"]["atproto"]["repo"]["putRecord"]>[0]
>;

function makeMetadataState(repo: unknown) {
  const state: Record<string, any> = {};
  const slice = createContentMetadataSlice(
    (next: any) => Object.assign(state, next),
    () => state as any,
    {} as any,
  );
  Object.assign(state, slice, {
    pdsAgent: { com: { atproto: { repo } } },
    oauthSession: { did: "did:plc:creator" },
  });
  return state as typeof slice;
}

describe("content metadata requests", () => {
  it.each([
    ["createContentMetadata", "com.atproto.repo.createRecord"],
    ["updateContentMetadata", "com.atproto.repo.putRecord"],
  ] as const)(
    "sends an explicit self key for %s with empty metadata",
    async (action, endpoint) => {
      const requests: { path: string; body: PutRecordInput }[] = [];
      const agent = new Agent(async (path, init) => {
        const body = (await new Response(init.body).json()) as PutRecordInput;
        requests.push({ path, body });
        return new Response(
          JSON.stringify({
            uri: `at://${body.repo}/${body.collection}/${body.rkey}`,
            cid: "bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku",
          }),
          { headers: { "Content-Type": "application/json" } },
        );
      });
      const state = makeMetadataState(agent.com.atproto.repo);

      await state[action]({});

      expect(requests).toEqual([
        {
          path: `/xrpc/${endpoint}`,
          body: {
            repo: "did:plc:creator",
            collection: "place.stream.metadata.configuration",
            rkey: "self",
            record: {
              $type: "place.stream.metadata.configuration",
              createdAt: expect.any(String),
            },
          },
        },
      ]);
      expect(state.lastCreatedRecord.uri).toBe(
        "at://did:plc:creator/place.stream.metadata.configuration/self",
      );
    },
  );
});

describe("updateContentMetadata", () => {
  it.each([undefined, "3lch2y4p2kc2a"])(
    "updates the self record across repeated saves with rkey %s",
    async (rkey) => {
      const records = new Map<string, unknown>();
      const state = makeMetadataState({
        putRecord: async (input: PutRecordInput) => {
          const uri = `at://${input.repo}/${input.collection}/${input.rkey}`;
          records.set(uri, input.record);
          return { data: { uri, cid: "metadata-cid" } };
        },
      });
      const params = {
        rkey,
        contentWarnings: ["violence"],
        contentRights: { copyrightNotice: "Creator" },
        distributionPolicy: { deleteAfter: 300 },
      };

      await state.updateContentMetadata(params);
      expect(state.lastCreatedRecord.record.contentWarnings).toEqual({
        warnings: ["violence"],
      });
      await state.updateContentMetadata({ ...params, contentWarnings: [] });

      const uri =
        "at://did:plc:creator/place.stream.metadata.configuration/self";
      expect([...records.keys()]).toEqual([uri]);
      expect(records.get(uri)).toMatchObject({
        $type: "place.stream.metadata.configuration",
        contentRights: params.contentRights,
        distributionPolicy: params.distributionPolicy,
      });
      expect(records.get(uri)).not.toHaveProperty("contentWarnings");
      expect(state.lastCreatedRecord).toMatchObject({
        uri,
        cid: "metadata-cid",
        record: records.get(uri),
      });
      expect(state.updating).toBe(false);
      expect(state.error).toBeNull();
    },
  );

  it("reports failed writes without replacing the last saved record", async () => {
    const failure = new Error("Write rejected");
    const state = makeMetadataState({
      putRecord: async () => {
        throw failure;
      },
    });
    const previousRecord = {
      uri: "at://did:plc:creator/place.stream.metadata.configuration/self",
    };
    state.lastCreatedRecord = previousRecord;

    await expect(state.updateContentMetadata({})).rejects.toBe(failure);

    expect(state.updating).toBe(false);
    expect(state.error).toBe("Write rejected");
    expect(state.lastCreatedRecord).toBe(previousRecord);
  });
});
