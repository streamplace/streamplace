import type { LucideProps } from "lucide-react-native";
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { Icon } from "../src/components/ui/icons";

// The SVG is an input fixture with overlapping strokes. Only the native View
// binding is adapted; the icon wrapper and DOM composition remain real.
vi.mock("react-native", () => ({
  View: ({ children, style }: any) => createElement("div", { style }, children),
}));
vi.mock("../src/lib/theme/theme", () => ({
  useTheme: () => ({ theme: { colors: { secondaryForeground: "#ffffff" } } }),
}));

function NavigationIcon({ color, size, strokeWidth = 2 }: LucideProps) {
  return (
    <svg width={size} height={size} strokeWidth={strokeWidth}>
      <path d="M4 12h16" stroke={color} />
      <circle cx={12} cy={12} r={8} stroke={color} fill="none" />
    </svg>
  );
}

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;
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
  ["rgba(255,255,255,0.45)", "rgba(255,255,255,1)", "0.45"],
  ["rgba(9, 9, 11, 0.66)", "rgba(9, 9, 11,1)", "0.66"],
  ["rgba(100,50,200,0)", "rgba(100,50,200,1)", "0"],
  ["#ff0088", "#ff0088", "1"],
  ["currentColor", "currentColor", "1"],
])(
  "composites %s once across overlapping icon paths",
  async (color, stroke, opacity) => {
    await act(async () =>
      root.render(<Icon icon={NavigationIcon} color={color} />),
    );
    const svg = container.querySelector("svg")!;
    expect(svg.parentElement!.style.opacity).toBe(opacity);
    const paths = svg.querySelectorAll("path,circle");
    expect(paths.length).toBeGreaterThan(1);
    for (const path of paths) {
      expect(path.getAttribute("stroke")).toBe(stroke);
      expect(path.getAttribute("stroke-opacity")).toBeNull();
    }
  },
);

it("preserves element props when tinting a supplied navigation icon", async () => {
  await act(async () =>
    root.render(
      <Icon
        icon={<NavigationIcon strokeWidth={3} />}
        color="rgba(255,255,255,0.65)"
        size={24}
      />,
    ),
  );
  const svg = container.querySelector("svg")!;
  expect(svg.getAttribute("width")).toBe("24");
  expect(svg.getAttribute("stroke-width")).toBe("3");
  expect(svg.parentElement!.style.opacity).toBe("0.65");
});
