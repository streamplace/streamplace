import {
  handleWebSocketMessages,
  makeLivestreamStore,
} from "@streamplace/core";
import { describe, expect, it } from "vitest";
import {
  moderationPermissionsFor,
  moderatorRecordsFromListRecords,
  permissionRecordsFromListRecords,
  type ModerationPermissionRecord,
} from "./moderation";

const STREAMER = "did:plc:streamer";
const MODERATOR = "did:plc:moderator";

function permissionRecord(
  overrides: Record<string, unknown> = {},
): Record<string, unknown> {
  return {
    $type: "place.stream.moderation.permission",
    moderator: MODERATOR,
    permissions: [],
    createdAt: "2024-01-01T00:00:00.000Z",
    ...overrides,
  };
}

describe("permissionRecordsFromListRecords", () => {
  it("keeps permission records and drops everything else", () => {
    const records = permissionRecordsFromListRecords([
      { value: permissionRecord() },
      { value: { $type: "place.stream.chat.message", text: "hi" } },
      { value: null },
    ]);
    expect(records).toHaveLength(1);
    expect(records[0].moderator).toBe(MODERATOR);
    expect(records[0].uri).toBeUndefined();

    const recordsWithURI = permissionRecordsFromListRecords([
      { value: permissionRecord(), uri: "at://streamer/permission/3kq" },
    ]);
    expect(recordsWithURI[0].uri).toBe("at://streamer/permission/3kq");
  });
});

describe("moderatorRecordsFromListRecords", () => {
  it("derives each record's rkey from its URI", () => {
    const records = moderatorRecordsFromListRecords([
      {
        value: permissionRecord(),
        uri: "at://did:plc:streamer/place.stream.moderation.permission/3kq",
      },
    ]);
    expect(records).toHaveLength(1);
    expect(records[0].rkey).toBe("3kq");
    expect(records[0].moderator).toBe(MODERATOR);
  });

  it("drops non-permission records and keeps the rest addressable", () => {
    const records = moderatorRecordsFromListRecords([
      { value: permissionRecord(), uri: "at://x/permission/aaa" },
      { value: { $type: "place.stream.chat.message", text: "hi" } },
      { value: null },
    ]);
    expect(records.map((r) => r.rkey)).toEqual(["aaa"]);
  });

  it("falls back to an empty rkey when no URI is present", () => {
    const records = moderatorRecordsFromListRecords([
      { value: permissionRecord() },
    ]);
    expect(records[0].rkey).toBe("");
  });
});

describe("moderationPermissionsFor", () => {
  it("grants everything to the streamer", () => {
    const perms = moderationPermissionsFor([], STREAMER, STREAMER);
    expect(perms).toEqual({
      canBan: true,
      canHide: true,
      canPin: true,
      canManageLivestream: true,
      isOwner: true,
    });
  });

  it("grants nothing to anonymous viewers", () => {
    const perms = moderationPermissionsFor([], null, STREAMER);
    expect(perms).toEqual({
      canBan: false,
      canHide: false,
      canPin: false,
      canManageLivestream: false,
      isOwner: false,
    });
  });

  it("grants nothing without a streamer", () => {
    const perms = moderationPermissionsFor([], MODERATOR, null);
    expect(perms.isOwner).toBe(false);
    expect(perms.canBan).toBe(false);
  });

  it("maps delegated permission strings to flags", () => {
    const records = [
      permissionRecord({
        permissions: ["ban", "message.pin"],
      }) as ModerationPermissionRecord,
    ];
    const perms = moderationPermissionsFor(records, MODERATOR, STREAMER);
    expect(perms).toEqual({
      canBan: true,
      canHide: false,
      canPin: true,
      canManageLivestream: false,
      isOwner: false,
    });
  });

  it("merges permissions across multiple delegation records", () => {
    const records = [
      permissionRecord({ permissions: ["ban"] }),
      permissionRecord({
        permissions: ["hide", "livestream.manage"],
        createdAt: "2024-01-02T00:00:00.000Z",
      }),
    ] as ModerationPermissionRecord[];
    const perms = moderationPermissionsFor(records, MODERATOR, STREAMER);
    expect(perms.canBan).toBe(true);
    expect(perms.canHide).toBe(true);
    expect(perms.canManageLivestream).toBe(true);
  });

  it("ignores delegations for other moderators", () => {
    const records = [
      permissionRecord({
        moderator: "did:plc:someoneelse",
        permissions: ["ban"],
      }),
    ] as ModerationPermissionRecord[];
    const perms = moderationPermissionsFor(records, MODERATOR, STREAMER);
    expect(perms.canBan).toBe(false);
  });

  it("ignores expired delegations", () => {
    const records = [
      permissionRecord({
        permissions: ["ban"],
        expirationTime: "2020-01-01T00:00:00.000Z",
      }),
    ] as ModerationPermissionRecord[];
    const perms = moderationPermissionsFor(records, MODERATOR, STREAMER);
    expect(perms.canBan).toBe(false);
  });

  it("honors unexpired delegations", () => {
    const future = new Date(Date.now() + 60 * 60 * 1000).toISOString();
    const records = [
      permissionRecord({ permissions: ["ban"], expirationTime: future }),
    ] as ModerationPermissionRecord[];
    const perms = moderationPermissionsFor(records, MODERATOR, STREAMER);
    expect(perms.canBan).toBe(true);
  });

  it("removes a moderator's controls when the websocket revokes their permission", () => {
    const store = makeLivestreamStore();
    const permission = permissionRecord({
      permissions: ["ban", "hide", "message.pin", "livestream.manage"],
      uri: "at://did:plc:streamer/place.stream.moderation.permission/3kq",
    }) as ModerationPermissionRecord;
    store.setState({ moderationPermissions: [permission] });

    expect(
      moderationPermissionsFor(
        store.getState().moderationPermissions,
        MODERATOR,
        STREAMER,
      ).canBan,
    ).toBe(true);

    store.setState((state) =>
      handleWebSocketMessages(state, [
        {
          $type: "place.stream.moderation.permission",
          deleted: true,
          uri: permission.uri,
        },
      ]),
    );

    expect(store.getState().moderationPermissions).toEqual([]);
    expect(
      moderationPermissionsFor(
        store.getState().moderationPermissions,
        MODERATOR,
        STREAMER,
      ),
    ).toMatchObject({
      canBan: false,
      canHide: false,
      canPin: false,
      canManageLivestream: false,
      isOwner: false,
    });
  });
});
