// @vitest-environment node
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
type GetRecordInput = NonNullable<
  Parameters<StreamplaceAgent["com"]["atproto"]["repo"]["getRecord"]>[0]
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
        if (init.method?.toUpperCase() === "GET") {
          return new Response(
            JSON.stringify({
              error: "RecordNotFound",
              message: "Record not found",
            }),
            { status: 400, headers: { "Content-Type": "application/json" } },
          );
        }
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
  it("preserves offline rights and distribution settings through live warning edits", async () => {
    const uri = "at://did:plc:creator/place.stream.metadata.configuration/self";
    let savedRecord: PutRecordInput["record"] = {};
    const saveRecord = async (input: PutRecordInput) => {
      savedRecord = structuredClone(input.record);
      return { data: { uri, cid: "metadata-cid" } };
    };
    const repo = {
      createRecord: saveRecord,
      putRecord: saveRecord,
      getRecord: async (input: GetRecordInput) => {
        if (`at://${input.repo}/${input.collection}/${input.rkey}` !== uri) {
          throw Object.assign(new Error("Record not found"), {
            error: "RecordNotFound",
          });
        }
        return {
          data: {
            uri,
            cid: "metadata-cid",
            value: structuredClone(savedRecord),
          },
        };
      },
    };
    const offlineState = makeMetadataState(repo);
    const contentRights = {
      copyrightNotice: "Creator",
      license: "https://creativecommons.org/licenses/by/4.0/",
    };
    const distributionPolicy = {
      deleteAfter: 300,
      allowedBroadcasters: ["did:plc:broadcaster"],
    };
    await offlineState.createContentMetadata({
      contentRights,
      distributionPolicy,
    });
    const liveState = makeMetadataState(repo);
    const livestreamRef = {
      uri: "at://did:plc:creator/place.stream.livestream/3lch2y4p2kc2a",
      cid: "livestream-cid",
    };

    await liveState.updateContentMetadata({
      livestreamRef,
      contentWarnings: ["violence"],
    });

    expect(savedRecord).toMatchObject({
      contentRights,
      distributionPolicy,
      livestreamRef,
      contentWarnings: { warnings: ["violence"] },
    });

    await liveState.updateContentMetadata({
      livestreamRef,
      contentWarnings: [],
    });

    expect(savedRecord).toMatchObject({ contentRights, distributionPolicy });
    expect(savedRecord).not.toHaveProperty("contentWarnings");
    expect(liveState.lastCreatedRecord).toMatchObject({
      uri,
      record: savedRecord,
    });
  });

  it("clears explicitly empty rights and distribution settings without changing warnings", async () => {
    let savedRecord: PutRecordInput["record"] = {
      contentWarnings: { warnings: ["violence"] },
      contentRights: { copyrightNotice: "Creator" },
      distributionPolicy: { deleteAfter: 300 },
    };
    const state = makeMetadataState({
      getRecord: async () => ({
        data: { value: structuredClone(savedRecord) },
      }),
      putRecord: async (input: PutRecordInput) => {
        savedRecord = input.record;
        return { data: { cid: "metadata-cid" } };
      },
    });

    await state.updateContentMetadata({
      contentRights: {},
      distributionPolicy: {},
    });

    expect(savedRecord).not.toHaveProperty("contentRights");
    expect(savedRecord).not.toHaveProperty("distributionPolicy");
    expect(savedRecord.contentWarnings).toEqual({ warnings: ["violence"] });
  });

  it("does not overwrite saved metadata when reading the current record fails", async () => {
    const failure = new Error("PDS unavailable");
    const putRecord = vi.fn();
    const state = makeMetadataState({
      getRecord: async () => {
        throw failure;
      },
      putRecord,
    });
    const previousRecord = { record: { contentRights: { license: "CC0" } } };
    state.lastCreatedRecord = previousRecord;

    await expect(
      state.updateContentMetadata({ contentWarnings: ["violence"] }),
    ).rejects.toBe(failure);

    expect(putRecord).not.toHaveBeenCalled();
    expect(state.lastCreatedRecord).toBe(previousRecord);
    expect(state.updating).toBe(false);
    expect(state.error).toBe("PDS unavailable");
  });

  it.each([undefined, "3lch2y4p2kc2a"])(
    "updates the self record across repeated saves with rkey %s",
    async (rkey) => {
      const records = new Map<string, unknown>();
      const state = makeMetadataState({
        getRecord: async (input: GetRecordInput) => {
          const uri = `at://${input.repo}/${input.collection}/${input.rkey}`;
          const value = records.get(uri);
          if (!value) {
            throw Object.assign(new Error("Record not found"), {
              error: "RecordNotFound",
            });
          }
          return { data: { value } };
        },
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
      getRecord: async () => ({ data: { value: {} } }),
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
