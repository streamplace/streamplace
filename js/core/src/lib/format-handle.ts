import { AppBskyActorDefs } from "@atproto/api";

/**
 * formats a user's handle for display, falling back to DID if handle is invalid
 */
export function formatHandle(
  profile: Pick<AppBskyActorDefs.ProfileViewBasic, "handle" | "did">,
  prefix: string = "",
): string {
  if (profile.handle === "handle.invalid") {
    return profile.did;
  }
  return prefix + profile.handle;
}

/**
 * convenience function for formatting a user's handle with @ prefix for display,
 * falling back to DID if handle is invalid
 */
export function formatHandleWithAt(
  profile: Pick<AppBskyActorDefs.ProfileViewBasic, "handle" | "did">,
): string {
  return formatHandle(profile, "@");
}
