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
});

describe("audioTrackLevels", () => {
  const stems = [
    { alternateGroup: 2, label: "Music" },
    { alternateGroup: 3, label: "Voice" },
  ];

  it("plays only the full mix by default", () => {
    expect(audioTrackLevels(stems, null)).toEqual([1, 0, 0]);
  });

  it("silences the full mix and plays every stem once stems are mixed", () => {
    expect(audioTrackLevels(stems, { 2: 0.25 })).toEqual([0, 0.25, 1]);
  });

  it("keeps the full mix when the stream has no stems", () => {
    expect(audioTrackLevels([], { 2: 0.25 })).toEqual([1]);
  });
});
