import type { LivestreamModerationPermission } from "@streamplace/core";
import type { StreamplaceAgent } from "streamplace";

export type ModerationPermissionRecord = LivestreamModerationPermission;

export const MODERATION_PERMISSION_COLLECTION =
  "place.stream.moderation.permission";

export interface ModerationPermissions {
  canBan: boolean;
  canHide: boolean;
  canPin: boolean;
  canManageLivestream: boolean;
  isOwner: boolean;
}

/**
 * Filter raw `com.atproto.repo.listRecords` results down to valid moderation
 * permission record values.
 */
export function permissionRecordsFromListRecords(
  records: Array<{ value: unknown; uri?: string } | null | undefined>,
): ModerationPermissionRecord[] {
  return records.flatMap((record) => {
    const value = record?.value;
    if (
      !value ||
      typeof value !== "object" ||
      (value as { $type?: unknown }).$type !== MODERATION_PERMISSION_COLLECTION
    ) {
      return [];
    }

    return [
      {
        ...(value as ModerationPermissionRecord),
        ...(record.uri ? { uri: record.uri } : {}),
      },
    ];
  });
}

/** A permission record paired with its rkey, addressing the record in the repo. */
export interface ModeratorRecord extends ModerationPermissionRecord {
  rkey: string;
}

/**
 * Map raw `com.atproto.repo.listRecords` results to moderator records. Records
 * without a URI (which `listRecords` always provides) get an empty rkey and are
 * therefore not addressable for removal.
 */
export function moderatorRecordsFromListRecords(
  records: Array<{ value: unknown; uri?: string } | null | undefined>,
): ModeratorRecord[] {
  return permissionRecordsFromListRecords(records).map((record) => ({
    ...record,
    rkey: record.uri?.split("/").pop() ?? "",
  }));
}

/** List every moderation permission record in a repo, following all cursors. */
export async function listModerationPermissionRecords(
  agent: StreamplaceAgent,
  repo: string,
): Promise<ModeratorRecord[]> {
  const records: Array<{ value: unknown; uri?: string }> = [];
  let cursor: string | undefined;

  do {
    const result = await agent.com.atproto.repo.listRecords({
      repo,
      collection: MODERATION_PERMISSION_COLLECTION,
      limit: 100,
      cursor,
    });
    records.push(...(result.data.records ?? []));
    cursor = result.data.cursor;
  } while (cursor);

  return moderatorRecordsFromListRecords(records);
}

/** Revoke every permission record the repo has granted to a moderator. */
export async function removeModeratorDelegations(
  agent: StreamplaceAgent,
  repo: string,
  moderatorDid: string,
): Promise<void> {
  const records = await listModerationPermissionRecords(agent, repo);
  for (const record of records) {
    if (record.moderator !== moderatorDid || !record.rkey) continue;
    await agent.com.atproto.repo.deleteRecord({
      repo,
      collection: MODERATION_PERMISSION_COLLECTION,
      rkey: record.rkey,
    });
  }
}

/**
 * Resolve a viewer's moderation capabilities for a streamer from the
 * streamer's delegation records. The streamer always has full permissions;
 * everyone else gets the union of their unexpired delegation records' perms.
 */
export function moderationPermissionsFor(
  records: ModerationPermissionRecord[],
  userDid: string | null | undefined,
  streamerDid: string | null | undefined,
): ModerationPermissions {
  const isOwner = !!userDid && !!streamerDid && userDid === streamerDid;
  if (isOwner || !userDid || !streamerDid) {
    return {
      canBan: isOwner,
      canHide: isOwner,
      canPin: isOwner,
      canManageLivestream: isOwner,
      isOwner,
    };
  }

  const granted = new Set<string>();
  for (const record of records) {
    if (record.moderator !== userDid) continue;
    if (
      record.expirationTime &&
      new Date(record.expirationTime) <= new Date()
    ) {
      continue;
    }
    for (const perm of record.permissions ?? []) {
      granted.add(perm);
    }
  }

  return {
    canBan: granted.has("ban"),
    canHide: granted.has("hide"),
    canPin: granted.has("message.pin"),
    canManageLivestream: granted.has("livestream.manage"),
    isOwner: false,
  };
}
