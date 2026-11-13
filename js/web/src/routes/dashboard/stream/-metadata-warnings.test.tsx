import { DashboardStoreContext } from "@/components/dashboard/dashboard-store-context";
import { SessionProvider } from "@/lib/session";
import { useStore } from "@/lib/store";
import type { LivestreamStore } from "@streamplace/core";
import { createInstance } from "i18next";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider, initReactI18next } from "react-i18next";
import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { createStore } from "zustand/vanilla";
import { StreamSettingsPage } from ".";

const warning = "place.stream.metadata.contentWarnings#death";
const initialStore = useStore.getState();
const i18n = createInstance();

vi.hoisted(() => {
  Object.defineProperty(navigator, "locks", {
    configurable: true,
    value: {
      request: () => {
        throw new Error("Unexpected lock request");
      },
    },
  });
});

beforeAll(async () => {
  await i18n.use(initReactI18next).init({ lng: "en", resources: { en: {} } });
});

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
});

afterEach(() => {
  useStore.setState(initialStore, true);
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
});

describe("stream metadata warnings", () => {
  it("keeps cleared warnings empty when the livestream still has a warning", async () => {
    const liveStore = createStore(() => ({
      livestream: { record: { contentWarnings: { warnings: [warning] } } },
    })) as unknown as LivestreamStore;
    useStore.setState({
      oauthSession: { did: "did:plc:creator" } as never,
      lastCreatedRecord: { record: null },
      pdsAgent: null,
    });

    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    const render = () => (
      <I18nextProvider i18n={i18n}>
        <SessionProvider>
          <DashboardStoreContext.Provider value={liveStore}>
            <StreamSettingsPage />
          </DashboardStoreContext.Provider>
        </SessionProvider>
      </I18nextProvider>
    );
    await act(async () => root.render(render()));
    const checkbox = Array.from(container.querySelectorAll("label"))
      .find((el) => el.textContent?.includes("Death"))
      ?.querySelector("input");
    expect(checkbox?.checked).toBe(true);
    await act(async () => checkbox?.click());
    expect(checkbox?.checked).toBe(false);

    await act(async () => {
      useStore.setState({
        lastCreatedRecord: {
          record: { contentWarnings: { warnings: [warning] } },
        },
      });
    });
    expect(checkbox?.checked).toBe(false);

    await act(async () => {
      useStore.setState({ lastCreatedRecord: { record: {} } });
      root.unmount();
    });
    const reloaded = createRoot(container);
    await act(async () => reloaded.render(render()));
    const afterReload = Array.from(container.querySelectorAll("label"))
      .find((el) => el.textContent?.includes("Death"))
      ?.querySelector("input");
    expect(afterReload?.checked).toBe(false);
    await act(async () => reloaded.unmount());
  });
});
