import type { LivestreamState } from "./state";

export function getActivePinnedComment(state: LivestreamState) {
  const { pinnedComment, livestream } = state;
  const scope = pinnedComment?.record.livestream;
  // Snapshots can deliver the pin before its livestream. Keep the received
  // record in the store so a matching chapter can make it visible later.
  if (
    scope &&
    (!livestream ||
      livestream.record.endedAt !== undefined ||
      (livestream.record.broadcast ?? livestream.uri) !== scope)
  ) {
    return null;
  }
  return pinnedComment;
}
