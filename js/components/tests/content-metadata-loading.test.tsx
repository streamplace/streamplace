import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { createStore } from "zustand/vanilla";
import { ContentMetadataForm } from "../src/components/content-metadata/content-metadata-form";
import { StreamplaceContext } from "../src/streamplace-provider/context";

function View({ children }: { children?: ReactNode }) {
  return createElement("div", null, children);
}
vi.mock("react-native", () => ({
  View,
  ScrollView: View,
  Pressable: View,
  Linking: { openURL: async () => {} },
  Platform: { OS: "web", select: (value: any) => value.web },
}));
vi.mock("lucide-react-native", () => ({ ExternalLink: View }));
vi.mock("../src/ui", async () => ({
  ...(await import("../src/lib/theme/atoms")),
  useTheme: () => ({ theme: { colors: { primary: "black" } } }),
}));
vi.mock("../src/components/ui", () => ({
  Admonition: View,
  SegmentedTabs: View,
}));
vi.mock("../src/components/ui/text", () => ({ Text: View }));
vi.mock("../src/components/ui/tooltip", () => ({ Tooltip: View }));
vi.mock("../src/components/ui/toast", () => ({
  useToast: () => ({ show: () => {} }),
}));
vi.mock("../src/components/ui/button", () => ({
  Button: ({ children, onPress, disabled }: any) =>
    createElement("button", { onClick: onPress, disabled }, children),
}));
vi.mock("../src/components/ui/checkbox", () => ({
  Checkbox: ({ label, checked, onCheckedChange }: any) =>
    createElement(
      "label",
      null,
      label,
      createElement("input", {
        type: "checkbox",
        checked,
        onChange: (e: any) => onCheckedChange(e.target.checked),
      }),
    ),
}));
vi.mock("../src/components/ui/input", () => ({ Input: () => null }));
vi.mock("../src/components/ui/select", () => ({ Select: () => null }));
vi.mock("../src/components/ui/textarea", () => ({ Textarea: () => null }));
vi.mock("../src/storage", () => ({ default: {} }));

const did = "did:plc:creator";
const warning = "place.stream.metadata.contentWarnings#death";
const cid = "bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku";
const uri = `at://${did}/place.stream.metadata.configuration/self`;
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

function pds(initial: Record<string, unknown> | null) {
  let record = initial;
  let fail = false;
  let wait = Promise.resolve();
  const response = (body: unknown, status = 200) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  const session = {
    did,
    fetchHandler: async (path: string, init: RequestInit) => {
      const pathname = new URL(path, "http://pds.test").pathname;
      if (pathname.endsWith(".getBroadcaster"))
        return response({ broadcaster: "did:web:node.example" });
      if (pathname.endsWith(".getRecord")) {
        await wait;
        if (fail)
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
      if (
        pathname.endsWith(".putRecord") ||
        pathname.endsWith(".createRecord")
      ) {
        record = (
          (await new Response(init.body).json()) as {
            record: Record<string, unknown>;
          }
        ).record;
        return response({ uri, cid });
      }
      throw new Error(`Unexpected request ${path}`);
    },
  };
  return {
    session,
    get record() {
      return record;
    },
    set fail(value: boolean) {
      fail = value;
    },
    set wait(value: Promise<void>) {
      wait = value;
    },
  };
}

async function render(pdsState: ReturnType<typeof pds>) {
  const store = createStore(() => ({
    oauthSession: pdsState.session,
    broadcasterDID: "did:web:node.example",
    setBroadcasterDID: () => {},
    setServerDID: () => {},
  }));
  await act(async () =>
    root.render(
      <StreamplaceContext.Provider value={{ store: store as never }}>
        <ContentMetadataForm />
      </StreamplaceContext.Provider>,
    ),
  );
}
function button(label: string) {
  return Array.from(container.querySelectorAll("button")).find(
    (el) => el.textContent?.trim() === label,
  );
}

it("blocks editing until metadata loads, then saves without losing other fields", async () => {
  const original = {
    $type: "place.stream.metadata.configuration",
    contentWarnings: { warnings: [warning] },
    contentRights: { copyrightNotice: "Creator" },
    distributionPolicy: {
      allowedBroadcasters: ["did:web:node.example"],
      deleteAfter: 600,
    },
  };
  const state = pds(original);
  let release!: () => void;
  state.wait = new Promise<void>((resolve) => {
    release = resolve;
  });
  await render(state);
  expect(button("Save Metadata")?.disabled ?? true).toBe(true);
  expect(container.querySelector('input[type="checkbox"]')).toBeNull();
  await act(async () => release());
  expect(button("Update Metadata")?.disabled).toBe(false);
  const checkbox = Array.from(container.querySelectorAll("label"))
    .find((el) => el.textContent === "Death")
    ?.querySelector("input");
  expect(checkbox?.checked).toBe(true);
  await act(async () => checkbox?.click());
  await act(async () => button("Update Metadata")?.click());
  expect(state.record).toMatchObject({
    contentRights: original.contentRights,
    distributionPolicy: original.distributionPolicy,
  });
  expect(state.record).not.toHaveProperty("contentWarnings");
});

it("blocks saving after a failed read and allows retry", async () => {
  const state = pds({
    $type: "place.stream.metadata.configuration",
    contentWarnings: { warnings: [warning] },
  });
  state.fail = true;
  await render(state);
  expect(container.textContent).toContain("Couldn't load metadata");
  expect(button("Save Metadata")?.disabled ?? true).toBe(true);
  state.fail = false;
  await act(async () => button("Retry")?.click());
  expect(button("Update Metadata")?.disabled).toBe(false);
});

it("allows defaults after the PDS confirms there is no record", async () => {
  await render(pds(null));
  expect(button("Save Metadata")?.disabled).toBe(false);
});
