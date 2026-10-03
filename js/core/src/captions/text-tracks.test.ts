import { describe, expect, it } from "vitest";
import { watchTextTracks } from "./text-tracks";
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
