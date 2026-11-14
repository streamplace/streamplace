import { DashboardStoreContext } from "@/components/dashboard/dashboard-store-context";
import { SessionProvider } from "@/lib/session";
import { useStore } from "@/lib/store";
import type { LivestreamStore } from "@streamplace/core";
import { createInstance } from "i18next";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { StreamplaceAgent } from "streamplace";
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

const did = "did:plc:creator";
const warning = "place.stream.metadata.contentWarnings#death";
const uri = `at://${did}/place.stream.metadata.configuration/self`;
const cid = "bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku";
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

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.stubGlobal("fetch", async () => new Response(null, { status: 503 }));
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  useStore.setState(initialStore, true);
  container.remove();
  vi.unstubAllGlobals();
});

function metadataPds(initial: Record<string, unknown> | null) {
  let record = initial;
  let failure = false;
  let wait: Promise<void> = Promise.resolve();
  const response = (body: unknown, status = 200) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  const agent = new StreamplaceAgent(async (path, init) => {
    const pathname = new URL(path, "http://pds.test").pathname;
    if (pathname.endsWith("/place.stream.ingest.getIngestUrls"))
      return response({ ingests: [] });
    if (pathname.endsWith(".getRecord")) {
      await wait;
      if (failure)
        return response(
          { error: "ServiceUnavailable", message: "PDS not found" },
          503,
        );
      if (!record)
        return response(
          { error: "RecordNotFound", message: "Record not found" },
          400,
        );
      return response({ uri, cid, value: record });
    }
    if (pathname.endsWith(".putRecord") || pathname.endsWith(".createRecord")) {
      record = (
        (await new Response(init.body).json()) as {
          record: Record<string, unknown>;
        }
      ).record;
      return response({ uri, cid });
    }
    throw new Error(`Unexpected request: ${path}`);
  });
  return {
    agent,
    get record() {
      return record;
    },
    set failure(value: boolean) {
      failure = value;
    },
    set wait(value: Promise<void>) {
      wait = value;
    },
  };
}

async function render(pds: ReturnType<typeof metadataPds>) {
  const liveStore = createStore(() => ({
    livestream: {
      uri: `at://${did}/place.stream.livestream/live`,
      cid: "live-cid",
      record: { title: "Live", createdAt: "2026-01-01T00:00:00Z" },
    },
  })) as unknown as LivestreamStore;
  useStore.setState({
    oauthSession: { did } as never,
    pdsAgent: pds.agent as never,
    lastCreatedRecord: null,
    error: null,
  });
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <SessionProvider>
          <DashboardStoreContext.Provider value={liveStore}>
            <StreamSettingsPage />
          </DashboardStoreContext.Provider>
        </SessionProvider>
      </I18nextProvider>,
    ),
  );
}

function button(label: string) {
  return Array.from(container.querySelectorAll("button")).find(
    (el) => el.textContent?.trim() === label,
  );
}
function warningCheckbox() {
  return Array.from(container.querySelectorAll("label"))
    .find((el) => el.textContent?.includes("Death"))
    ?.querySelector("input");
}

describe("stream metadata loading", () => {
  it("waits for saved metadata before allowing edits and preserves other settings when warnings are cleared", async () => {
    const original = {
      $type: "place.stream.metadata.configuration",
      contentWarnings: { warnings: [warning] },
      contentRights: { creator: "Creator", copyrightNotice: "Creator" },
      distributionPolicy: {
        allowedBroadcasters: ["did:web:node.example"],
        deleteAfter: 600,
      },
    };
    const pds = metadataPds(original);
    let release!: () => void;
    pds.wait = new Promise<void>((resolve) => {
      release = resolve;
    });
    await render(pds);
    expect(button("Save")?.disabled ?? true).toBe(true);
    expect(warningCheckbox()).toBeUndefined();
    await act(async () => release());
    expect(warningCheckbox()?.checked).toBe(true);
    await act(async () => warningCheckbox()?.click());
    await act(async () => button("Save")?.click());
    expect(pds.record).toMatchObject({
      contentRights: original.contentRights,
      distributionPolicy: original.distributionPolicy,
    });
    expect(pds.record).not.toHaveProperty("contentWarnings");
  });

  it("keeps editing blocked on a failed read and loads the saved record on retry", async () => {
    const pds = metadataPds({ contentWarnings: { warnings: [warning] } });
    pds.failure = true;
    await render(pds);
    expect(button("Save")?.disabled ?? true).toBe(true);
    expect(container.textContent).toContain("Couldn't load metadata");
    pds.failure = false;
    await act(async () => button("Retry")?.click());
    expect(warningCheckbox()?.checked).toBe(true);
    expect(button("Save")?.disabled).toBe(false);
  });

  it("allows defaults after the PDS confirms there is no record", async () => {
    const pds = metadataPds(null);
    await render(pds);
    expect(warningCheckbox()?.checked).toBe(false);
    expect(button("Save")?.disabled).toBe(false);
  });
});
