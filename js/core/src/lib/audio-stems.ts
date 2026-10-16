import { place } from "streamplace";

export type AudioStem = { alternateGroup: number; label: string };

/**
 * The playable stems in a segment's audio: tracks of the same alternateGroup
 * carry the same sound in different codecs, the lowest group (or an entry
 * without one) is the full mix. WebRTC sends up to two higher groups with an
 * Opus encode, in ascending order.
 */
export function audioStems(
  segment: place.stream.segment.Main | null | undefined,
): AudioStem[] {
  const audio = segment?.audio ?? [];
  const main = Math.min(...audio.map((a) => a.alternateGroup ?? 0));
  const opusGroups = new Set(
    audio.filter((a) => a.codec === "opus").map((a) => a.alternateGroup ?? 0),
  );
  // Each group's label, from whichever of its entries has one.
  const stems = new Map<number, string>();
  for (const a of audio) {
    const group = a.alternateGroup ?? 0;
    if (group > main && opusGroups.has(group) && !stems.get(group)) {
      stems.set(group, a.label ?? "");
    }
  }
  return [...stems]
    .sort(([a], [b]) => a - b)
    .slice(0, 2)
    .map(([alternateGroup, label]) => ({
      alternateGroup,
      label: label || `Track ${alternateGroup}`,
    }));
}

/**
 * A received WebRTC audio track's group: null for the full mix, the stem's
 * alternateGroup, or undefined while unknown.
 */
export type AudioTrackGroup = number | null | undefined;

/**
 * The level (0-1, before the player volume) of each received WebRTC audio
 * track, given its group. By default only the full mix plays; once the viewer
 * sets stem volumes (keyed by alternateGroup, 1 when unset), the full mix is
 * silent and every stem the stream still carries plays at its own level.
 * Levels follow the track's group, not its position: a connection keeps the
 * stems it was answered with while the stream's stems may change.
 */
export function audioTrackLevels(
  trackGroups: AudioTrackGroup[],
  stems: AudioStem[],
  stemVolumes: Record<number, number> | null,
): number[] {
  const mixing = stemVolumes !== null && stems.length > 0;
  return trackGroups.map((group) => {
    if (group === null) {
      return mixing ? 0 : 1;
    }
    if (
      group === undefined ||
      stemVolumes === null ||
      !stems.some((s) => s.alternateGroup === group)
    ) {
      return 0;
    }
    return stemVolumes[group] ?? 1;
  });
}
