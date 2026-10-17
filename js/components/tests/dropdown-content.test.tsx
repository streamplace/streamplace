import type { ReactNode } from "react";
import { act, createElement, useState } from "react";
import type { Root } from "react-dom/client";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ErrorBoundary } from "../src/components/error-boundary";
import { DropdownContent } from "../src/components/ui/dropdown-content";

// Native presentation bindings only; the menu content and both error boundaries
// are real. Portal placement/opening is covered by the native Maestro flow.
vi.mock("react-native", () => {
  const View = ({
    children,
    testID,
  }: {
    children: ReactNode;
    testID?: string;
  }) => createElement("div", { "data-testid": testID }, children);
  return { View, Text: View, Platform: { OS: "android", select: () => null } };
});
vi.mock("../src/components/ui/button", () => {
  return {
    Button: ({
      children,
      onPress,
    }: {
      children: ReactNode;
      onPress: () => void;
    }) => createElement("button", { onClick: onPress }, children),
  };
});
vi.mock("../src/components/ui/text", () => {
  return {
    Text: ({ children }: { children: ReactNode }) =>
      createElement("span", null, children),
  };
});

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.spyOn(console, "error").mockImplementation(() => {});
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function click(label: string) {
  const button = [...container.querySelectorAll("button")].find(
    (element) => element.textContent === label,
  );
  expect(button, `button ${label}`).toBeDefined();
  await act(async () => button!.click());
}

it.each(["component", "render callback"])(
  "contains a failing %s and preserves the screen through retry, dismiss, and reopen",
  async (kind) => {
    let failing = true;
    function Content() {
      if (failing) throw new Error("profile render failed");
      return <span>Recovered profile</span>;
    }
    function Screen() {
      const [draft, setDraft] = useState("Unsent message");
      const [open, setOpen] = useState(false);
      return (
        <>
          <button onClick={() => setDraft("Edited unsent message")}>
            {draft}
          </button>
          <button onClick={() => setOpen(true)}>Open menu</button>
          {open && (
            <DropdownContent
              stack={[
                {
                  key: "profile",
                  content: kind === "component" ? <Content /> : () => Content(),
                },
              ]}
              onDismiss={() => setOpen(false)}
            />
          )}
        </>
      );
    }
    await act(async () => {
      root.render(
        <ErrorBoundary fallback={<span>App crashed</span>}>
          <Screen />
        </ErrorBoundary>,
      );
    });
    await click("Unsent message");
    await click("Open menu");
    expect(
      container.querySelector('[data-testid="menu-error"]'),
    ).not.toBeNull();
    expect(container.textContent).not.toContain("App crashed");
    expect(container.textContent).toContain("Edited unsent message");

    // A persistent failure remains local, and retry can later recover in place.
    await click("Try again");
    expect(
      container.querySelector('[data-testid="menu-error"]'),
    ).not.toBeNull();
    failing = false;
    await click("Try again");
    expect(container.querySelector('[data-testid="menu-error"]')).toBeNull();
    expect(container.textContent).toContain("Recovered profile");
    expect(container.textContent).toContain("Edited unsent message");

    // Fail again on a subsequent render, dismiss the failed menu, then reopen
    // without inheriting the old boundary's failed state or resetting the draft.
    failing = true;
    await act(async () => {
      root.render(
        <ErrorBoundary fallback={<span>App crashed</span>}>
          <Screen />
        </ErrorBoundary>,
      );
    });
    expect(
      container.querySelector('[data-testid="menu-error"]'),
    ).not.toBeNull();
    await click("Dismiss");
    expect(container.querySelector('[data-testid="menu-error"]')).toBeNull();
    expect(container.textContent).toContain("Edited unsent message");
    failing = false;
    await click("Open menu");
    expect(container.textContent).toContain("Recovered profile");
    expect(container.textContent).toContain("Edited unsent message");
    expect(container.textContent).not.toContain("App crashed");
  },
);
