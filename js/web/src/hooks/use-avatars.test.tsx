import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import useAvatars from "./use-avatars";

const { state } = vi.hoisted(() => ({
  state: { getProfiles: vi.fn(), profileCache: {} as Record<string, any> },
}));
vi.mock("../lib/store", () => ({
  useStore: (selector: any) => selector(state),
}));
vi.mock("../lib/store/hooks", () => ({
  useCachedProfiles: () => state.profileCache,
}));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  state.getProfiles.mockReset();
  state.profileCache = {};
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});
function Probe({ actors }: { actors: string[] }) {
  const profiles = useAvatars(actors);
  return <span>{Object.keys(profiles).join(",")}</span>;
}
it("does not refetch omitted actors on cache updates or playback renders", async () => {
  const actor = "did:plc:not-on-bluesky";
  await act(async () => root.render(<Probe actors={[actor]} />));
  expect(state.getProfiles).toHaveBeenCalledTimes(1);
  // A successful Bluesky response can omit a non-Bluesky actor. Its empty
  // cache update, and a new array from every playback render, must not loop.
  for (let i = 0; i < 5; i++) {
    state.profileCache = {};
    await act(async () => root.render(<Probe actors={[actor]} />));
  }
  expect(state.getProfiles).toHaveBeenCalledTimes(1);
  await act(async () => root.render(<Probe actors={["did:plc:new-viewer"]} />));
  expect(state.getProfiles).toHaveBeenLastCalledWith(["did:plc:new-viewer"]);
  expect(state.getProfiles).toHaveBeenCalledTimes(2);
});
