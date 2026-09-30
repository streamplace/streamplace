import type { LivestreamView } from "@/hooks/use-live-users";
import type { TeleportErrorData } from "@streamplace/core";
import { act, type ButtonHTMLAttributes, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TeleportDialog } from "./teleport-dialog";

const { mockUseLiveUsers } = vi.hoisted(() => ({
  mockUseLiveUsers: vi.fn(),
}));

vi.mock("@/hooks/use-live-users", () => ({
  useLiveUsers: mockUseLiveUsers,
}));
vi.mock("@/hooks/use-avatars", () => ({
  default: () => ({}),
}));
vi.mock("@/components/ui/button", () => ({
  Button: ({ children, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props}>{children}</button>
  ),
}));
vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogContent: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
  DialogDescription: ({ children }: { children: ReactNode }) => (
    <p>{children}</p>
  ),
  DialogFooter: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
  DialogHeader: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
  DialogTitle: ({ children }: { children: ReactNode }) => <h2>{children}</h2>,
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

const liveStream = {
  uri: "at://did:plc:streamer/place.stream.livestream/stream",
  author: { did: "did:plc:streamer", handle: "streamer.example" },
  record: { title: "Live now" },
} as unknown as LivestreamView;

describe("TeleportDialog", () => {
  let container: HTMLDivElement;
  let root: Root;

  function render(
    onSubmit: () => Promise<TeleportErrorData | undefined> = async () =>
      undefined,
  ) {
    root.render(
      <TeleportDialog open onOpenChange={() => {}} onSubmit={onSubmit} />,
    );
  }

  beforeEach(() => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    mockUseLiveUsers.mockReturnValue({ data: [liveStream], isLoading: false });
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    mockUseLiveUsers.mockReset();
    vi.unstubAllGlobals();
  });

  it("clears a streamer selection when that stream goes offline", async () => {
    await act(async () => render());
    const streamerButton = Array.from(
      container.querySelectorAll("button"),
    ).find((button) => button.textContent?.includes("@streamer.example"));
    expect(streamerButton).toBeDefined();

    await act(async () => streamerButton?.click());
    const teleportButton = Array.from(
      container.querySelectorAll("button"),
    ).find((button) => button.textContent === "teleport-start");
    expect(teleportButton?.disabled).toBe(false);

    mockUseLiveUsers.mockReturnValue({ data: [], isLoading: false });
    await act(async () => render());

    expect(container.querySelector('[role="alert"]')?.textContent).toBe(
      "teleport-selection-expired",
    );
    const currentTeleportButton = Array.from(
      container.querySelectorAll("button"),
    ).find((button) => button.textContent === "teleport-start");
    expect(currentTeleportButton?.disabled).toBe(true);
  });

  it("translates typed teleport errors", async () => {
    await act(async () => render(async () => ({ code: "self" })));
    const streamerButton = Array.from(
      container.querySelectorAll("button"),
    ).find((button) => button.textContent?.includes("@streamer.example"));
    await act(async () => streamerButton?.click());
    const teleportButton = Array.from(
      container.querySelectorAll("button"),
    ).find((button) => button.textContent === "teleport-start");

    await act(async () => teleportButton?.click());

    expect(container.querySelector('[role="alert"]')?.textContent).toBe(
      "teleport-error-self",
    );
  });
});
