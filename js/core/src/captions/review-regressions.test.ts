import { place } from "streamplace";
import { describe, expect, it } from "vitest";
import { timedCaptionsAt } from "./api";
import {
  activeLiveCaptions,
  displayLiveCaptions,
  reduceLiveCaption,
} from "./live-cues";
import { watchTextTracks } from "./text-tracks";

const track = {
  id: "human-en",
  language: "en",
  source: "human",
  origin: "canonical",
};
const cue = (
  start: number,
  end: number,
): place.stream.caption.defs.LiveCue => ({
  id: "cue",
  track,
  text: "Speech",
  final: true,
  startTime: new Date(start).toISOString(),
  endTime: new Date(end).toISOString(),
});

describe("caption presentation boundaries", () => {
  it("shows live speech only within its media interval, not its arrival window", () => {
    const cues = reduceLiveCaption({}, cue(10000, 30000), 50000);
    expect(activeLiveCaptions(cues, track.id, 9000)).toEqual([]);
    expect(
      activeLiveCaptions(cues, track.id, 20000).map((c) => c.text),
    ).toEqual(["Speech"]);
    expect(activeLiveCaptions(cues, track.id, 30000)).toEqual([]);
  });
  it("keeps delayed OBS speech readable on arrival and replaces it with newer speech", () => {
    let cues = reduceLiveCaption({}, cue(10000, 30000), 50000);
    expect(
      displayLiveCaptions(cues, track.id, 56000).map((c) => c.text),
    ).toEqual(["Speech"]);
    expect(displayLiveCaptions(cues, track.id, 70000)).toEqual([]);
    cues = reduceLiveCaption(
      cues,
      { ...cue(31000, 32000), id: "next", text: "Next speech" },
      57000,
    );
    expect(
      displayLiveCaptions(cues, track.id, 58000).map((c) => c.text),
    ).toEqual(["Next speech"]);
    expect(displayLiveCaptions(cues, track.id, 62000)).toEqual([]);
  });
  it("finds a long-running VOD cue behind many expired overlapping cues", () => {
    const cues = [
      { startMs: 0, endMs: 60000, text: "Long speech" },
      ...Array.from({ length: 12 }, (_, i) => ({
        startMs: (i + 1) * 1000,
        endMs: (i + 1) * 1000 + 500,
        text: "Short",
      })),
    ];
    expect(timedCaptionsAt(cues, 15000).map((c) => c.text)).toEqual([
      "Long speech",
    ]);
  });
});

class Track extends EventTarget {
  kind = "subtitles";
  language = "en";
  label = "English";
  mode = "disabled";
  activeCues = [
    {
      text: "<v Speaker>A &amp; B &lt;3</v>",
      getCueAsHTML: () => ({ textContent: "A & B <3" }),
    },
  ];
}
class Tracks extends EventTarget {
  items: Track[] = [];
  [Symbol.iterator]() {
    return this.items[Symbol.iterator]();
  }
}

describe("browser TextTrack presentation", () => {
  it("renders browser-parsed WebVTT text rather than escaped payload markup", () => {
    const tracks = new Tracks();
    tracks.items = [new Track()];
    let lines: string[] = [];
    const watcher = watchTextTracks(
      { textTracks: tracks } as unknown as HTMLVideoElement,
      () => {},
      (next) => {
        lines = next;
      },
    );
    watcher.setActive("0");
    expect(lines).toEqual(["A & B <3"]);
    watcher.dispose();
  });
  it("does not react to removed tracks after disposal", () => {
    const tracks = new Tracks();
    const track = new Track();
    tracks.items = [track];
    let changes = 0;
    const watcher = watchTextTracks(
      { textTracks: tracks } as unknown as HTMLVideoElement,
      () => {},
      () => {
        changes++;
      },
    );
    watcher.setActive("0");
    tracks.items = [];
    tracks.dispatchEvent(new Event("removetrack"));
    watcher.dispose();
    tracks.items = [track];
    const before = changes;
    track.activeCues = [
      { text: "changed", getCueAsHTML: () => ({ textContent: "changed" }) },
    ];
    track.dispatchEvent(new Event("cuechange"));
    expect(changes).toBe(before);
  });
});
