import { place } from "streamplace";
import { describe, expect, it } from "vitest";
import { audioStems, audioTrackLevels } from "./audio-stems";

const segment = (audio: place.stream.segment.Audio[]) =>
  ({ audio }) as place.stream.segment.Main;

describe("audioStems", () => {
  it("finds no stems without alternate groups", () => {
    expect(
      audioStems(segment([{ codec: "opus", rate: 48000, channels: 2 }])),
    ).toEqual([]);
    expect(audioStems(null)).toEqual([]);
  });

  it("lists the groups above the full mix once each, in group order", () => {
    expect(
      audioStems(
        segment([
          { codec: "aac", rate: 48000, channels: 2, alternateGroup: 3 },
          { codec: "aac", rate: 48000, channels: 2, alternateGroup: 1 },
          {
            codec: "aac",
            rate: 48000,
            channels: 2,
            alternateGroup: 4,
            label: "Voice",
          },
          {
            codec: "opus",
            rate: 48000,
            channels: 2,
            alternateGroup: 3,
            label: "Music",
          },
          {
            codec: "opus",
            rate: 48000,
            channels: 2,
            alternateGroup: 4,
            label: "Voice",
          },
        ]),
      ),
    ).toEqual([
      { alternateGroup: 3, label: "Music" },
      { alternateGroup: 4, label: "Voice" },
    ]);
  });

  it("treats an entry without a group as the full mix", () => {
    expect(
      audioStems(
        segment([
          { codec: "opus", rate: 48000, channels: 2 },
          { codec: "opus", rate: 48000, channels: 2, alternateGroup: 2 },
        ]),
      ),
    ).toEqual([{ alternateGroup: 2, label: "Track 2" }]);
  });

  it("skips AAC-only groups before taking the first two playable stems", () => {
    expect(
      audioStems(
        segment([
          { codec: "opus", rate: 48000, channels: 2, alternateGroup: 1 },
          {
            codec: "aac",
            rate: 48000,
            channels: 2,
            alternateGroup: 2,
            label: "AAC only",
          },
          {
            codec: "opus",
            rate: 48000,
            channels: 2,
            alternateGroup: 5,
            label: "Third",
          },
          {
            codec: "opus",
            rate: 48000,
            channels: 2,
            alternateGroup: 4,
            label: "Second",
          },
          {
            codec: "opus",
            rate: 48000,
            channels: 2,
            alternateGroup: 3,
            label: "First",
          },
        ]),
      ),
    ).toEqual([
      { alternateGroup: 3, label: "First" },
      { alternateGroup: 4, label: "Second" },
    ]);
  });
});

describe("audioTrackLevels", () => {
  const stems = [
    { alternateGroup: 2, label: "Music" },
    { alternateGroup: 3, label: "Voice" },
  ];
  const tracks = [null, 2, 3];

  it("plays only the full mix by default", () => {
    expect(audioTrackLevels(tracks, stems, null)).toEqual([1, 0, 0]);
  });

  it("silences the full mix and plays every stem once stems are mixed", () => {
    expect(audioTrackLevels(tracks, stems, { 2: 0.25 })).toEqual([0, 0.25, 1]);
  });

  it("keeps the full mix when the stream has no stems", () => {
    expect(audioTrackLevels([null], [], { 2: 0.25 })).toEqual([1]);
  });

  // A connection answered with groups 2 and 3 keeps them after the stream
  // drops group 2: its second stem track is still group 3, and must keep
  // group 3's level rather than take the first stem slot's.
  it("follows each track's group when the stream's stems change", () => {
    expect(
      audioTrackLevels(tracks, [{ alternateGroup: 3, label: "Voice" }], {
        3: 0.5,
      }),
    ).toEqual([0, 0, 0.5]);
  });

  it("silences a track whose group is not known yet", () => {
    expect(audioTrackLevels([null, undefined], stems, { 2: 1 })).toEqual([
      0, 0,
    ]);
  });
});
