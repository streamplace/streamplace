import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { Avatar } from "../src/components/ui/avatar";
import {
  spacing,
  statusColors,
  surfaces,
  textAlphas,
} from "../src/lib/theme/tokens";

function View({ children, style }: any) {
  return (
    <div
      data-style={JSON.stringify(Object.assign({}, ...[style].flat(Infinity)))}
    >
      {children}
    </div>
  );
}
vi.mock("react-native", () => ({
  View,
  Text: ({ children }: any) => <span>{children}</span>,
}));
vi.mock("expo-image", () => ({ Image: () => null }));
vi.mock("../src/lib/theme/theme", () => ({
  useTheme: () => ({
    theme: {
      spacing,
      colors: {
        live: statusColors.live,
        surface3: surfaces.dark[3],
        text2: textAlphas.dark[2],
      },
    },
  }),
}));

let root: ReturnType<typeof createRoot>;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});

it.each([
  ["xs", 20, 1.25],
  ["md", 32, 2],
  ["2xl", 64, 4],
  [42, 42, 2.625],
] as const)(
  "scales the %s avatar's live ring and gap with its image",
  async (size, imageSide, ringWidth) => {
    await act(async () => root.render(<Avatar size={size} live />));
    const outer = JSON.parse(
      container.firstElementChild!.getAttribute("data-style")!,
    );
    const image = JSON.parse(
      container.firstElementChild!.firstElementChild!.getAttribute(
        "data-style",
      )!,
    );
    expect(image.width).toBe(imageSide);
    expect(outer.width).toBe(imageSide + ringWidth * 4);
    expect(outer.borderWidth).toBe(ringWidth);
    expect((outer.width - image.width) / 2 - outer.borderWidth).toBe(ringWidth);

    await act(async () => root.render(<Avatar size={size} />));
    const offline = JSON.parse(
      container.firstElementChild!.getAttribute("data-style")!,
    );
    expect(offline.width).toBe(imageSide);
    expect(offline.borderWidth).toBeUndefined();
  },
);
