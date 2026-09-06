import { describe, expect, it } from "vitest";
import {
  moderationPermissionsFor,
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
});
