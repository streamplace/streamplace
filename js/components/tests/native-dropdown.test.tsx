import { act, createElement, useState } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  DropdownMenuCheckboxItem,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSubTrigger,
} from "../src/components/ui/dropdown.native";

const theme = vi.hoisted(() => ({
  theme: { colors: {} },
  icons: { color: {} },
}));

// Native input/rendering is adapted to DOM events; menu selection and the
// shared button's event handling run unchanged.
vi.mock("react-native", () => {
  const View = ({ children }: any) => createElement("div", null, children);
  return {
    View,
    Text: View,
    Platform: { OS: "ios" },
    StyleSheet: {
      create: (styles: unknown) => styles,
      flatten: (styles: any) => Object.assign({}, ...[styles].flat(Infinity)),
    },
    Pressable: ({ children, disabled, onPress, style, ...props }: any) => {
      const [pressed, setPressed] = useState(false);
      return createElement(
        "button",
        {
          disabled,
          onClick: onPress,
          onMouseDown: () => !disabled && setPressed(true),
          onMouseUp: () => setPressed(false),
          onMouseLeave: () => setPressed(false),
          "aria-label": props.accessibilityLabel,
          "aria-checked": props.accessibilityState?.checked,
          style: Object.assign(
            {},
            ...[typeof style === "function" ? style({ pressed }) : style].flat(
              Infinity,
            ),
          ),
        },
        typeof children === "function" ? children({ pressed }) : children,
      );
    },
  };
});
vi.mock("@gorhom/bottom-sheet", () => ({ default: () => null }));
vi.mock("@rn-primitives/dropdown-menu", () => ({
  Root: () => null,
  Trigger: () => null,
  Portal: () => null,
}));
vi.mock("react-native-gesture-handler", () => ({}));
vi.mock("react-native-reanimated", () => ({ default: {} }));
vi.mock("react-native-safe-area-context", () => ({}));
vi.mock("lucide-react-native", () => ({
  Check: () => null,
  CheckCircle: () => null,
  ChevronLeft: () => null,
  ChevronRight: () => null,
  Circle: () => null,
}));
vi.mock("../src/ui", () => ({ useTheme: () => theme }));
vi.mock("../src/lib/theme/theme", () => ({ useTheme: () => theme }));
vi.mock("../src/components/ui/text", () => ({
  Text: ({ children }: any) => createElement("span", null, children),
}));
vi.mock("../src/components/ui/dropdown-content", () => ({}));

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

it.each([DropdownMenuItem, DropdownMenuSubTrigger])(
  "exposes the actual press state to a row's content",
  async (Item) => {
    await act(async () =>
      root.render(
        <Item>{({ pressed }) => (pressed ? "Pressed" : "Idle")}</Item>,
      ),
    );
    expect(container.textContent).toBe("Idle");
    const button = container.querySelector("button")!;
    await act(async () =>
      button.dispatchEvent(new MouseEvent("mousedown", { bubbles: true })),
    );
    expect(container.textContent).toBe("Pressed");
    await act(async () =>
      button.dispatchEvent(new MouseEvent("mouseup", { bubbles: true })),
    );
    expect(container.textContent).toBe("Idle");
  },
);

it.each([
  DropdownMenuItem,
  DropdownMenuSubTrigger,
  DropdownMenuCheckboxItem,
  DropdownMenuRadioItem,
])("evaluates a menu row's caller style with its press state", async (Item) => {
  await act(async () =>
    root.render(
      <Item style={({ pressed }) => ({ opacity: pressed ? 0.5 : 1 })}>
        Action
      </Item>,
    ),
  );
  const button = container.querySelector("button")!;
  expect(button.style.opacity).toBe("1");
  await act(async () =>
    button.dispatchEvent(new MouseEvent("mousedown", { bubbles: true })),
  );
  expect(button.style.opacity).toBe("0.5");
  await act(async () =>
    button.dispatchEvent(new MouseEvent("mouseup", { bubbles: true })),
  );
  expect(button.style.opacity).toBe("1");
});

it("prevents disabled menu actions", async () => {
  function Menu() {
    const [action, setAction] = useState("Unchanged");
    return (
      <DropdownMenuItem disabled onPress={() => setAction("Activated")}>
        {action}
      </DropdownMenuItem>
    );
  }
  await act(async () => root.render(<Menu />));
  await act(async () => container.querySelector("button")!.click());
  expect(container.textContent).toBe("Unchanged");
});

it("preserves a submenu trigger's additional press action", async () => {
  function Menu() {
    const [action, setAction] = useState("Idle");
    return (
      <DropdownMenuSubTrigger onPress={() => setAction("Activated")}>
        {action}
      </DropdownMenuSubTrigger>
    );
  }
  await act(async () => root.render(<Menu />));
  await act(async () => container.querySelector("button")!.click());
  expect(container.textContent).toBe("Activated");
});

it("updates a checkbox and delivers its additional press action", async () => {
  function Menu() {
    const [checked, setChecked] = useState(false);
    const [action, setAction] = useState("Idle");
    return (
      <DropdownMenuCheckboxItem
        checked={checked}
        onCheckedChange={setChecked}
        onPress={() => setAction("Activated")}
      >
        {checked ? "Enabled" : "Disabled"} {action}
      </DropdownMenuCheckboxItem>
    );
  }
  await act(async () => root.render(<Menu />));
  await act(async () => container.querySelector("button")!.click());
  expect(container.textContent).toBe("Enabled Activated");
  await act(async () => container.querySelector("button")!.click());
  expect(container.textContent).toBe("Disabled Activated");
});

it("selects a radio option and delivers its additional press action", async () => {
  function Menu() {
    const [value, setValue] = useState("source");
    const [action, setAction] = useState("Idle");
    return (
      <DropdownMenuRadioGroup value={value} onValueChange={setValue}>
        <DropdownMenuRadioItem
          value="audio"
          onPress={() => setAction("Activated")}
        >
          {value} {action}
        </DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
    );
  }
  await act(async () => root.render(<Menu />));
  await act(async () => container.querySelector("button")!.click());
  expect(container.textContent).toBe("audio Activated");
});
