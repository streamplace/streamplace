import type { place } from "streamplace";

export type ModerationPermissionRecord =
  place.stream.moderation.permission.Main;

const PERMISSION_RECORD_TYPE = "place.stream.moderation.permission";

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
  records: Array<{ value: unknown } | null | undefined>,
): ModerationPermissionRecord[] {
  return records
    .map((r) => r?.value)
    .filter(
      (value): value is ModerationPermissionRecord =>
        !!value &&
        typeof value === "object" &&
        (value as { $type?: unknown }).$type === PERMISSION_RECORD_TYPE,
    );
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
