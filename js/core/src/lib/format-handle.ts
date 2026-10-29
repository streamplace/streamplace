/**
 * formats a user's handle for display or routing, falling back to the bare DID
 * (never prefixed) when the handle is missing, empty, or the "handle.invalid"
 * sentinel the network returns for unverifiable handles
 */
export function formatHandle(
  profile: { did: string; handle?: string | null },
  prefix: string = "",
): string {
  if (!profile.handle || profile.handle === "handle.invalid") {
    return profile.did;
  }
  return prefix + profile.handle;
}

/**
 * convenience function for formatting a user's handle with @ prefix for display,
 * falling back to the bare DID if the handle is missing or invalid
 */
export function formatHandleWithAt(profile: {
  did: string;
  handle?: string | null;
}): string {
  return formatHandle(profile, "@");
}
