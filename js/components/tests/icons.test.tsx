import { act, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { Icon } from "../src/components/ui/icons";
import { darkTheme, lightTheme, ThemeProvider } from "../src/lib/theme/theme";

vi.mock("react-native", () => ({
  View: ({ children, style }: any) => <div style={style}>{children}</div>,
  Platform: { OS: "web", select: () => null },
  useColorScheme: () => "dark",
}));
vi.mock("react-native-gesture-handler", () => ({
  GestureHandlerRootView: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("@rn-primitives/portal", () => ({ PortalHost: () => null }));
vi.mock("../src/components/ui/toast", () => ({
  ToastProvider: ({ children }: { children: ReactNode }) => children,
}));

function Glyph({ color }: { color?: string }) {
  return <svg stroke={color} />;
}
function appearance(color: string, opacity = 1) {
  const rgba = color.match(/^rgba\((.+)\)$/);
  if (!rgba) return [color, opacity];
  const [red, green, blue, alpha] = rgba[1].split(",").map(Number);
  return [red, green, blue, alpha * opacity];
}
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

it.each(["light", "dark"] as const)(
  "uses icon foreground colors on %s surfaces and respects an explicit color",
  async (mode) => {
    const theme = mode === "dark" ? darkTheme : lightTheme;
    await act(async () =>
      root.render(
        <ThemeProvider forcedTheme={mode} portalHost={false}>
          <Icon icon={Glyph} />
          <Icon icon={Glyph} variant="muted" />
          <Icon icon={Glyph} color={theme.colors.danger} />
          <Icon icon={<Glyph />} variant="muted" />
        </ThemeProvider>,
      ),
    );
    const icons = [...container.querySelectorAll("svg")];
    expect(
      icons.map((icon) =>
        appearance(
          icon.getAttribute("stroke")!,
          Number(icon.parentElement!.style.opacity),
        ),
      ),
    ).toEqual(
      [
        theme.colors.text,
        theme.colors.textMuted,
        theme.colors.danger,
        theme.colors.textMuted,
      ].map((color) => appearance(color)),
    );
    for (const icon of icons) {
      const stroke = icon.getAttribute("stroke")!;
      if (stroke.startsWith("rgba(")) expect(appearance(stroke)[3]).toBe(1);
    }
  },
);
